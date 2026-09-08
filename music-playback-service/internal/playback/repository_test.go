package playback

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// testRepository connects to a real Postgres and self-skips without
// DB_URL, matching every other DB-gated test in this codebase
// (music-identity-gatekeeper's internal/event, internal/playlist, etc.).
func testRepository(t *testing.T) *PostgresRepository {
	t.Helper()
	dsn := os.Getenv("DB_URL")
	if dsn == "" {
		t.Skip("DB_URL not set, skipping DB-gated test")
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect to Postgres: %v", err)
	}
	t.Cleanup(pool.Close)

	return &PostgresRepository{db: pool}
}

func newTestEvent(userID uuid.UUID) *Event {
	return &Event{
		ID:            uuid.New(),
		ClientEventID: uuid.New(),
		UserID:        userID,
		SongID:        uuid.New(),
		SessionID:     uuid.New(),
		Type:          EventTypePlay,
		OccurredAt:    time.Now(),
		PositionMS:    0,
		DeviceType:    "mobile",
	}
}

func TestPostgresRepositoryInsertAndDuplicate(t *testing.T) {
	repo := testRepository(t)
	event := newTestEvent(uuid.New())

	inserted, err := repo.Insert(context.Background(), event)
	if err != nil {
		t.Fatalf("Insert() error = %v", err)
	}
	if !inserted {
		t.Fatalf("Insert() inserted = false, want true on first write")
	}
	firstID := event.ID

	// Resubmit the exact same (user_id, client_event_id): must report
	// duplicate and hand back the original row's ID, not create a second one.
	retry := *event
	retry.ID = uuid.New() // a genuine client retry would generate a fresh server-side attempt too
	retry.PositionMS = 999

	inserted2, err2 := repo.Insert(context.Background(), &retry)
	if err2 != nil {
		t.Fatalf("Insert() retry error = %v", err2)
	}
	if inserted2 {
		t.Fatalf("Insert() retry inserted = true, want false (duplicate)")
	}
	if retry.ID != firstID {
		t.Fatalf("Insert() retry ID = %s, want original %s", retry.ID, firstID)
	}

	var count int
	err = repo.db.QueryRow(context.Background(),
		`SELECT count(*) FROM playback_events WHERE user_id = $1 AND client_event_id = $2`,
		event.UserID, event.ClientEventID,
	).Scan(&count)
	if err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("row count = %d, want exactly 1 ledger row after retry", count)
	}
}

func TestPostgresRepositoryConcurrentInsertIsIdempotent(t *testing.T) {
	repo := testRepository(t)
	userID := uuid.New()
	clientEventID := uuid.New()

	const attempts = 10
	var wg sync.WaitGroup
	insertedCount := make(chan bool, attempts)

	for range attempts {
		wg.Go(func() {
			event := &Event{
				ID:            uuid.New(),
				ClientEventID: clientEventID,
				UserID:        userID,
				SongID:        uuid.New(),
				SessionID:     uuid.New(),
				Type:          EventTypePlay,
				OccurredAt:    time.Now(),
				DeviceType:    "mobile",
			}
			inserted, err := repo.Insert(context.Background(), event)
			if err != nil {
				t.Errorf("Insert() error = %v", err)
				return
			}
			insertedCount <- inserted
		})
	}
	wg.Wait()
	close(insertedCount)

	trueCount := 0
	for inserted := range insertedCount {
		if inserted {
			trueCount++
		}
	}
	if trueCount != 1 {
		t.Fatalf("exactly one concurrent Insert() should report inserted=true, got %d", trueCount)
	}

	var count int
	if err := repo.db.QueryRow(context.Background(),
		`SELECT count(*) FROM playback_events WHERE user_id = $1 AND client_event_id = $2`,
		userID, clientEventID,
	).Scan(&count); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("row count = %d, want exactly 1 ledger row after concurrent retries", count)
	}
}

// --- detectOutOfOrder: pure unit tests, no DB needed ---

func TestDetectOutOfOrder(t *testing.T) {
	base := time.Now()
	id := func(n int) uuid.UUID {
		// Deterministic, distinct UUIDs for readable test failures.
		var u uuid.UUID
		u[0] = byte(n)
		return u
	}

	t.Run("strictly increasing ingested_at flags nothing", func(t *testing.T) {
		rows := []overviewRow{
			{ID: id(1), OccurredAt: base, IngestedAt: base.Add(1 * time.Second)},
			{ID: id(2), OccurredAt: base.Add(1 * time.Minute), IngestedAt: base.Add(2 * time.Second)},
			{ID: id(3), OccurredAt: base.Add(2 * time.Minute), IngestedAt: base.Add(3 * time.Second)},
		}
		flagged := detectOutOfOrder(rows)
		if len(flagged) != 0 {
			t.Fatalf("flagged = %v, want none", flagged)
		}
	})

	t.Run("single regression flags the regressing row", func(t *testing.T) {
		rows := []overviewRow{
			{ID: id(1), OccurredAt: base, IngestedAt: base.Add(10 * time.Minute)},
			{ID: id(2), OccurredAt: base.Add(1 * time.Minute), IngestedAt: base.Add(1 * time.Minute)}, // arrived before row 1
		}
		flagged := detectOutOfOrder(rows)
		if !flagged[id(2)] {
			t.Fatalf("flagged = %v, want id(2) flagged", flagged)
		}
		if flagged[id(1)] {
			t.Fatalf("flagged = %v, want id(1) NOT flagged", flagged)
		}
	})

	t.Run("multi-step regression flags every regressing row, not just the first", func(t *testing.T) {
		// A(ingested 10:00), B(9:00), C(9:30) sorted by occurred_at.
		// Adjacent-only comparison would miss C (9:30 > B's 9:00), but C
		// still arrived before A — the exact bug caught during design review.
		rows := []overviewRow{
			{ID: id(1), OccurredAt: base, IngestedAt: base.Add(10 * time.Minute)},                                    // A
			{ID: id(2), OccurredAt: base.Add(1 * time.Minute), IngestedAt: base.Add(9 * time.Minute)},                // B
			{ID: id(3), OccurredAt: base.Add(2 * time.Minute), IngestedAt: base.Add(9*time.Minute + 30*time.Second)}, // C
		}
		flagged := detectOutOfOrder(rows)
		if !flagged[id(2)] || !flagged[id(3)] {
			t.Fatalf("flagged = %v, want both id(2) and id(3) flagged", flagged)
		}
		if flagged[id(1)] {
			t.Fatalf("flagged = %v, want id(1) NOT flagged", flagged)
		}
	})

	t.Run("same occurred_at peer group never produces a false positive from tiebreak order", func(t *testing.T) {
		tie := base.Add(1 * time.Minute)
		rows := []overviewRow{
			{ID: id(1), OccurredAt: base, IngestedAt: base.Add(1 * time.Second)},
			// Two rows sharing the exact same occurred_at, with
			// intentionally scrambled ingested_at relative to each other —
			// neither should be flagged just for being "after" the other
			// within the same peer group.
			{ID: id(2), OccurredAt: tie, IngestedAt: base.Add(3 * time.Second)},
			{ID: id(3), OccurredAt: tie, IngestedAt: base.Add(2 * time.Second)},
			{ID: id(4), OccurredAt: base.Add(2 * time.Minute), IngestedAt: base.Add(4 * time.Second)},
		}
		flagged := detectOutOfOrder(rows)
		if len(flagged) != 0 {
			t.Fatalf("flagged = %v, want none (peer-group members must not be compared to each other)", flagged)
		}
	})

	t.Run("row within a peer group can still be flagged against an earlier group", func(t *testing.T) {
		tie := base.Add(1 * time.Minute)
		rows := []overviewRow{
			{ID: id(1), OccurredAt: base, IngestedAt: base.Add(10 * time.Minute)},
			{ID: id(2), OccurredAt: tie, IngestedAt: base.Add(1 * time.Second)}, // arrived before row 1 despite being a later occurred_at group
		}
		flagged := detectOutOfOrder(rows)
		if !flagged[id(2)] {
			t.Fatalf("flagged = %v, want id(2) flagged", flagged)
		}
	})

	t.Run("empty input", func(t *testing.T) {
		if flagged := detectOutOfOrder(nil); len(flagged) != 0 {
			t.Fatalf("flagged = %v, want none", flagged)
		}
	})
}

// --- GetSessionEvents: DB-gated integration tests ---

func insertSessionEvent(t *testing.T, repo *PostgresRepository, userID, sessionID uuid.UUID, occurredAt time.Time) *Event {
	t.Helper()
	event := newTestEvent(userID)
	event.SessionID = sessionID
	event.OccurredAt = occurredAt

	if _, err := repo.Insert(context.Background(), event); err != nil {
		t.Fatalf("Insert() error = %v", err)
	}
	return event
}

func TestPostgresRepositoryGetSessionEventsNotFound(t *testing.T) {
	repo := testRepository(t)
	userID := uuid.New()
	sessionID := uuid.New()

	t.Run("session never existed", func(t *testing.T) {
		_, _, _, err := repo.GetSessionEvents(context.Background(), userID, sessionID, nil, 10)
		if !errors.Is(err, ErrSessionNotFound) {
			t.Fatalf("GetSessionEvents() error = %v, want ErrSessionNotFound", err)
		}
	})

	t.Run("session exists but owned by another user", func(t *testing.T) {
		ownerID := uuid.New()
		otherSessionID := uuid.New()
		insertSessionEvent(t, repo, ownerID, otherSessionID, time.Now())

		_, _, _, err := repo.GetSessionEvents(context.Background(), uuid.New(), otherSessionID, nil, 10)
		if !errors.Is(err, ErrSessionNotFound) {
			t.Fatalf("GetSessionEvents() error = %v, want ErrSessionNotFound (same code path as never-existed)", err)
		}
	})
}

func TestPostgresRepositoryGetSessionEventsPaginatesEmptyFirstMiddleFinal(t *testing.T) {
	repo := testRepository(t)
	userID := uuid.New()
	sessionID := uuid.New()

	base := time.Now().Add(-1 * time.Hour)
	const total = 5
	inserted := make([]*Event, 0, total)
	for i := range total {
		inserted = append(inserted, insertSessionEvent(t, repo, userID, sessionID, base.Add(time.Duration(i)*time.Minute)))
	}

	// First page.
	page1, overview, next1, err := repo.GetSessionEvents(context.Background(), userID, sessionID, nil, 2)
	if err != nil {
		t.Fatalf("first page error = %v", err)
	}
	if overview.EventCount != total {
		t.Fatalf("EventCount = %d, want %d", overview.EventCount, total)
	}
	if len(page1) != 2 || page1[0].ID != inserted[0].ID || page1[1].ID != inserted[1].ID {
		t.Fatalf("first page = %+v, want events 0,1 in order", page1)
	}
	if next1 == nil {
		t.Fatal("first page next cursor = nil, want non-nil (more pages remain)")
	}

	// Middle page.
	page2, _, next2, err := repo.GetSessionEvents(context.Background(), userID, sessionID, next1, 2)
	if err != nil {
		t.Fatalf("middle page error = %v", err)
	}
	if len(page2) != 2 || page2[0].ID != inserted[2].ID || page2[1].ID != inserted[3].ID {
		t.Fatalf("middle page = %+v, want events 2,3 in order", page2)
	}
	if next2 == nil {
		t.Fatal("middle page next cursor = nil, want non-nil (one more event remains)")
	}

	// Final page.
	page3, _, next3, err := repo.GetSessionEvents(context.Background(), userID, sessionID, next2, 2)
	if err != nil {
		t.Fatalf("final page error = %v", err)
	}
	if len(page3) != 1 || page3[0].ID != inserted[4].ID {
		t.Fatalf("final page = %+v, want event 4 only", page3)
	}
	if next3 != nil {
		t.Fatalf("final page next cursor = %+v, want nil", next3)
	}

	// Empty page: a stale cursor past the last event of a real session
	// returns 200-shaped empty results, NOT ErrSessionNotFound. next3 is
	// nil (correctly — there IS no next page), so it can't be reused
	// here: passing nil would just re-run the first page. Build an
	// explicit cursor from the last event's own position instead, mirroring
	// a real client that cached a cursor and is now polling for anything
	// new (there isn't any).
	staleCursor := &Cursor{OccurredAt: inserted[total-1].OccurredAt, ID: inserted[total-1].ID}
	pageEmpty, overviewEmpty, nextEmpty, err := repo.GetSessionEvents(context.Background(), userID, sessionID, staleCursor, 2)
	if err != nil {
		t.Fatalf("empty page error = %v, want nil (session exists, just no more events)", err)
	}
	if len(pageEmpty) != 0 {
		t.Fatalf("empty page = %+v, want zero events", pageEmpty)
	}
	if nextEmpty != nil {
		t.Fatalf("empty page next cursor = %+v, want nil", nextEmpty)
	}
	if overviewEmpty.EventCount != total {
		t.Fatalf("empty page EventCount = %d, want %d (summary is session-wide, not page-scoped)", overviewEmpty.EventCount, total)
	}
}

func TestPostgresRepositoryGetSessionEventsExactLimitBoundary(t *testing.T) {
	repo := testRepository(t)
	userID := uuid.New()
	sessionID := uuid.New()

	base := time.Now().Add(-1 * time.Hour)
	const total = 3
	for i := range total {
		insertSessionEvent(t, repo, userID, sessionID, base.Add(time.Duration(i)*time.Minute))
	}

	events, _, next, err := repo.GetSessionEvents(context.Background(), userID, sessionID, nil, total)
	if err != nil {
		t.Fatalf("GetSessionEvents() error = %v", err)
	}
	if len(events) != total {
		t.Fatalf("len(events) = %d, want %d", len(events), total)
	}
	if next != nil {
		t.Fatalf("next cursor = %+v, want nil when event count exactly equals limit", next)
	}
}

func TestPostgresRepositoryGetSessionEventsRetainsOutOfOrderEvents(t *testing.T) {
	repo := testRepository(t)
	userID := uuid.New()
	sessionID := uuid.New()

	now := time.Now()
	// Insert in real chronological order (ingested_at = real insert time)
	// but with the SECOND event's occurred_at earlier than the first's —
	// a genuine out-of-order arrival, not a synthetic one. In the
	// occurred_at-sorted view, "second" (earlier occurred_at) comes
	// first and establishes the high-water-mark; "first" (later
	// occurred_at) comes next, and its ingested_at is BEHIND that mark
	// (it was, in real wall-clock time, ingested before "second" was) —
	// so "first" is the one that gets flagged, matching the exact same
	// pattern as TestDetectOutOfOrder's "single regression" case: the
	// later-occurred_at row is flagged when its ingested_at falls behind
	// an earlier-occurred_at row's.
	first := insertSessionEvent(t, repo, userID, sessionID, now)
	second := insertSessionEvent(t, repo, userID, sessionID, now.Add(-10*time.Minute))

	events, overview, _, err := repo.GetSessionEvents(context.Background(), userID, sessionID, nil, 10)
	if err != nil {
		t.Fatalf("GetSessionEvents() error = %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("len(events) = %d, want 2 — out-of-order events must be RETAINED, not dropped", len(events))
	}
	// Ordered by occurred_at ASC: second (earlier occurred_at) comes first.
	if events[0].ID != second.ID || events[1].ID != first.ID {
		t.Fatalf("events = %+v, want [second, first] ordered by occurred_at", events)
	}
	if !overview.OutOfOrder[first.ID] {
		t.Fatalf("OutOfOrder = %v, want first event flagged (its ingested_at fell behind second's, despite occurring later)", overview.OutOfOrder)
	}
	if overview.OutOfOrder[second.ID] {
		t.Fatalf("OutOfOrder = %v, want second event NOT flagged (nothing precedes it in occurred_at order)", overview.OutOfOrder)
	}
}
