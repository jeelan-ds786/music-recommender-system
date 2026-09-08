package playback

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	playbackevent "github.com/jeelan-ds786/music-recommender-system/music-playback-service/internal/event"
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

func newTestMessage(event *Event) playbackevent.Message {
	return playbackevent.Message{
		ID:              uuid.New(),
		PlaybackEventID: event.ID,
		Topic:           playbackevent.PlaybackTopic,
		Key:             event.UserID.String(),
		EventType:       string(event.Type),
		SchemaVersion:   playbackevent.SchemaVersion,
		Headers:         playbackevent.PlaybackHeaders(string(event.Type)),
		Payload:         []byte("protobuf"),
		OccurredAt:      event.OccurredAt,
	}
}

func TestPostgresRepositoryInsertAndDuplicate(t *testing.T) {
	repo := testRepository(t)
	event := newTestEvent(uuid.New())

	inserted, err := repo.Insert(context.Background(), event, newTestMessage(event))
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

	inserted2, err2 := repo.Insert(context.Background(), &retry, newTestMessage(&retry))
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
	if err := repo.db.QueryRow(context.Background(),
		`SELECT count(*) FROM outbox_events WHERE playback_event_id = $1`, firstID,
	).Scan(&count); err != nil {
		t.Fatalf("count outbox rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("outbox row count = %d, want exactly 1 after retry", count)
	}
}

func TestPostgresRepositoryOutboxFailureRollsBackLedger(t *testing.T) {
	repo := testRepository(t)
	first := newTestEvent(uuid.New())
	message := newTestMessage(first)
	if inserted, err := repo.Insert(context.Background(), first, message); err != nil || !inserted {
		t.Fatalf("seed Insert() = %t, %v", inserted, err)
	}

	second := newTestEvent(uuid.New())
	collidingMessage := newTestMessage(second)
	collidingMessage.ID = message.ID
	if inserted, err := repo.Insert(context.Background(), second, collidingMessage); err == nil || inserted {
		t.Fatalf("colliding Insert() = %t, %v, want outbox error", inserted, err)
	}

	var count int
	if err := repo.db.QueryRow(context.Background(),
		`SELECT count(*) FROM playback_events WHERE id = $1`, second.ID,
	).Scan(&count); err != nil {
		t.Fatalf("count rolled-back ledger row: %v", err)
	}
	if count != 0 {
		t.Fatalf("rolled-back ledger row count = %d, want 0", count)
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
			inserted, err := repo.Insert(context.Background(), event, newTestMessage(event))
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
