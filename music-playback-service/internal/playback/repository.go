package playback

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// checkViolation is the Postgres SQLSTATE for a CHECK constraint failure.
const checkViolation = "23514"

// contextCheckConstraint is playback_events' CHECK on context's stored
// size. Application-level validation (ValidateIngest) measures the raw
// wire bytes of the context field, but Postgres's jsonb type re-serializes
// on storage — e.g. it inserts a space after every ':' — so a compact,
// whitespace-free payload measured as exactly at the 4096-byte cap by
// ValidateIngest can still land over it once stored, tripping this
// constraint even though the application check passed. Rather than try to
// replicate jsonb's exact canonicalization in Go (fragile, and Postgres is
// the authoritative source of truth for what gets stored), Insert
// specifically detects this one constraint and reports it the same way a
// pre-insert validation failure would, instead of leaking a raw 500.
const contextCheckConstraint = "playback_events_context_check"

func mapContextCheckViolation(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == checkViolation && pgErr.ConstraintName == contextCheckConstraint {
		return ErrContextTooLarge
	}
	return err
}

type Repository interface {
	// Insert stores event, or — if (user_id, client_event_id) already
	// exists — leaves event.ID pointing at the existing row instead.
	// Returns true only when this call actually created a new row.
	Insert(ctx context.Context, event *Event) (inserted bool, err error)

	// GetSessionEvents returns one page of a session's events (ordered
	// (occurred_at, id) ASC), the session-wide overview (event count,
	// start/end, out-of-order flags computed across the WHOLE session,
	// not just this page), and the next page's cursor (nil on the last
	// page). Returns ErrSessionNotFound if no event owned by userID
	// exists for sessionID.
	GetSessionEvents(ctx context.Context, userID, sessionID uuid.UUID, cursor *Cursor, limit int) ([]Event, *SessionOverview, *Cursor, error)
}

type PostgresRepository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) Repository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) Insert(ctx context.Context, event *Event) (bool, error) {
	contextPayload := event.Context
	if len(contextPayload) == 0 {
		contextPayload = json.RawMessage("{}")
	}

	// ON CONFLICT DO NOTHING (not DO UPDATE) is required here: an UPDATE
	// — even a no-op one used purely to RETURNING the existing row, the
	// usual upsert-and-return-existing trick — would fire
	// playback_events_append_only and abort the whole insert. DO NOTHING
	// means a conflict simply returns zero rows, which we detect via
	// pgx.ErrNoRows and follow with a plain SELECT below.
	query := `
		INSERT INTO playback_events
			(id, client_event_id, user_id, song_id, session_id, event_type,
			 occurred_at, position_ms, duration_ms, device_type, context)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (user_id, client_event_id) DO NOTHING
		RETURNING id
	`

	var insertedID uuid.UUID
	err := r.db.QueryRow(ctx, query,
		event.ID,
		event.ClientEventID,
		event.UserID,
		event.SongID,
		event.SessionID,
		string(event.Type),
		event.OccurredAt,
		event.PositionMS,
		event.DurationMS,
		event.DeviceType,
		contextPayload,
	).Scan(&insertedID)

	if err == nil {
		return true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return false, mapContextCheckViolation(err)
	}

	existingQuery := `SELECT id FROM playback_events WHERE user_id = $1 AND client_event_id = $2`
	if err := r.db.QueryRow(ctx, existingQuery, event.UserID, event.ClientEventID).Scan(&event.ID); err != nil {
		return false, err
	}

	return false, nil
}

// overviewRow is the narrow projection scanned for the whole-session
// overview query — deliberately just the three columns detectOutOfOrder
// and the summary computation actually need.
type overviewRow struct {
	ID         uuid.UUID
	OccurredAt time.Time
	IngestedAt time.Time
}

// detectOutOfOrder walks rows already sorted by (occurred_at, id) and
// flags any whose ingested_at falls behind the max ingested_at seen among
// rows with a strictly EARLIER occurred_at — i.e. it arrived after an
// event that, per its own timestamp, hadn't happened yet.
//
// Rows sharing the exact same occurred_at (a real possibility — low-
// resolution client clocks, batched submission) are treated as one peer
// group: every member is compared against the high-water-mark from
// BEFORE the group started, never against each other, and the mark only
// advances to the group's own max once the whole group is processed.
// Without this, the arbitrary (occurred_at, id) tiebreak among
// simultaneous events (id is a random UUID, no temporal meaning) could
// flag a row as out-of-order purely because of how its random id
// happened to sort — a false positive with no factual basis.
func detectOutOfOrder(rows []overviewRow) map[uuid.UUID]bool {
	flagged := make(map[uuid.UUID]bool)

	var highWaterMark time.Time
	for i := 0; i < len(rows); {
		j := i
		for j < len(rows) && rows[j].OccurredAt.Equal(rows[i].OccurredAt) {
			j++
		}

		groupMax := highWaterMark
		for k := i; k < j; k++ {
			if rows[k].IngestedAt.Before(highWaterMark) {
				flagged[rows[k].ID] = true
			}
			if rows[k].IngestedAt.After(groupMax) {
				groupMax = rows[k].IngestedAt
			}
		}
		highWaterMark = groupMax

		i = j
	}

	return flagged
}

func (r *PostgresRepository) GetSessionEvents(
	ctx context.Context,
	userID, sessionID uuid.UUID,
	cursor *Cursor,
	limit int,
) ([]Event, *SessionOverview, *Cursor, error) {

	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, nil, nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Narrow, unpaginated, uncapped scan of the WHOLE session — only 3
	// columns, cheap even for a large session — needed because
	// out-of-order detection and the session summary are session-wide
	// properties, not page-scoped ones. Capping this would silently
	// corrupt EventCount/EndedAt/DurationMS and miss out-of-order events
	// past the cap; see DECISIONS_AND_GOTCHAS.md for the full reasoning.
	overviewRows, err := tx.Query(ctx, `
		SELECT id, occurred_at, ingested_at
		FROM playback_events
		WHERE session_id = $1 AND user_id = $2
		ORDER BY occurred_at ASC, id ASC
	`, sessionID, userID)
	if err != nil {
		return nil, nil, nil, err
	}

	var rows []overviewRow
	for overviewRows.Next() {
		var row overviewRow
		if err := overviewRows.Scan(&row.ID, &row.OccurredAt, &row.IngestedAt); err != nil {
			overviewRows.Close()
			return nil, nil, nil, err
		}
		rows = append(rows, row)
	}
	overviewRows.Close()
	if err := overviewRows.Err(); err != nil {
		return nil, nil, nil, err
	}

	if len(rows) == 0 {
		return nil, nil, nil, ErrSessionNotFound
	}

	overview := &SessionOverview{
		EventCount: len(rows),
		StartedAt:  rows[0].OccurredAt,
		EndedAt:    rows[len(rows)-1].OccurredAt,
		OutOfOrder: detectOutOfOrder(rows),
	}
	overview.DurationMS = overview.EndedAt.Sub(overview.StartedAt).Milliseconds()

	// Paginated page of full event rows — same fetch-limit+1-then-trim
	// idiom as music-identity-gatekeeper's preference.ListLikedSongs,
	// ascending instead of descending. context is deliberately not
	// selected — not needed for operational verification.
	const baseQuery = `
		SELECT id, client_event_id, song_id, event_type, occurred_at,
		       position_ms, duration_ms, device_type, ingested_at
		FROM playback_events
		WHERE session_id = $1 AND user_id = $2
	`

	var pageRows pgx.Rows
	if cursor != nil {
		pageRows, err = tx.Query(ctx, baseQuery+`
			AND (occurred_at, id) > ($3, $4)
			ORDER BY occurred_at ASC, id ASC
			LIMIT $5
		`, sessionID, userID, cursor.OccurredAt, cursor.ID, limit+1)
	} else {
		pageRows, err = tx.Query(ctx, baseQuery+`
			ORDER BY occurred_at ASC, id ASC
			LIMIT $3
		`, sessionID, userID, limit+1)
	}
	if err != nil {
		return nil, nil, nil, err
	}

	events := make([]Event, 0, limit+1)
	for pageRows.Next() {
		var e Event
		var eventType string
		if err := pageRows.Scan(
			&e.ID, &e.ClientEventID, &e.SongID, &eventType, &e.OccurredAt,
			&e.PositionMS, &e.DurationMS, &e.DeviceType, &e.IngestedAt,
		); err != nil {
			pageRows.Close()
			return nil, nil, nil, err
		}
		e.Type = EventType(eventType)
		e.UserID = userID
		e.SessionID = sessionID
		events = append(events, e)
	}
	pageRows.Close()
	if err := pageRows.Err(); err != nil {
		return nil, nil, nil, err
	}

	var next *Cursor
	if len(events) > limit {
		events = events[:limit]
		last := events[limit-1]
		next = &Cursor{OccurredAt: last.OccurredAt, ID: last.ID}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, nil, err
	}

	return events, overview, next, nil
}
