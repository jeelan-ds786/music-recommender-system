package playback

import (
	"context"
	"encoding/json"
	"errors"

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
