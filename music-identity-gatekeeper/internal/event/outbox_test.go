package event

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/db"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/logger"
)

func setupOutboxTestDB(t *testing.T) *Outbox {
	dsn := os.Getenv("DB_URL")
	if dsn == "" {
		t.Skip("DB_URL not set, skipping DB-backed test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	log := logger.New(logger.LevelNone)
	pool, err := db.NewPostgresPool(ctx, dsn, db.NewQueryTracer(log))
	if err != nil {
		t.Fatalf("failed to connect to DB: %v", err)
	}

	_, err = pool.Exec(ctx, "TRUNCATE kafka_integration RESTART IDENTITY")
	if err != nil {
		t.Fatalf("failed to truncate table: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
	})

	return NewOutbox(pool, log)
}

func TestOutbox_EnqueueAndFetchPending(t *testing.T) {
	outbox := setupOutboxTestDB(t)
	ctx := context.Background()

	id, err := outbox.Enqueue(ctx, nil, "test.topic", "key1", []byte(`{"msg":"hello"}`))
	if err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}
	if id <= 0 {
		t.Errorf("expected a positive ID, got %d", id)
	}

	events, err := outbox.FetchPending(ctx, 10)
	if err != nil {
		t.Fatalf("FetchPending failed: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	e := events[0]
	if e.Topic != "test.topic" {
		t.Errorf("Topic = %q, want %q", e.Topic, "test.topic")
	}
	if e.Key != "key1" {
		t.Errorf("Key = %q, want %q", e.Key, "key1")
	}
	assertJSONEqual(t, e.Payload, []byte(`{"msg":"hello"}`))
	if e.Status != "pending" {
		t.Errorf("Status = %q, want %q", e.Status, "pending")
	}
	if e.Attempts != 0 {
		t.Errorf("Attempts = %d, want 0", e.Attempts)
	}
}

func TestOutbox_MarkPublishedAndFailed(t *testing.T) {
	outbox := setupOutboxTestDB(t)
	ctx := context.Background()

	// payload is a JSONB column — must be valid JSON, not an arbitrary string.
	_, _ = outbox.Enqueue(ctx, nil, "topic", "1", []byte(`{"n":"a"}`))
	_, _ = outbox.Enqueue(ctx, nil, "topic", "2", []byte(`{"n":"b"}`))
	_, _ = outbox.Enqueue(ctx, nil, "topic", "3", []byte(`{"n":"c"}`))

	events, _ := outbox.FetchPending(ctx, 10)
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}

	// Mark 1 published
	if err := outbox.MarkPublished(ctx, events[0].ID); err != nil {
		t.Fatalf("MarkPublished failed: %v", err)
	}

	// Mark 2 failed
	if err := outbox.MarkFailed(ctx, events[1].ID); err != nil {
		t.Fatalf("MarkFailed failed: %v", err)
	}

	// Increment attempts for 3
	if err := outbox.IncrementAttempts(ctx, events[2].ID); err != nil {
		t.Fatalf("IncrementAttempts failed: %v", err)
	}

	// Fetch again
	pending, _ := outbox.FetchPending(ctx, 10)
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending event, got %d", len(pending))
	}

	if pending[0].ID != events[2].ID {
		t.Errorf("expected pending event ID %d, got %d", events[2].ID, pending[0].ID)
	}
	if pending[0].Attempts != 1 {
		t.Errorf("expected attempts = 1, got %d", pending[0].Attempts)
	}
}

func TestOutbox_GetByID_ReturnsRowRegardlessOfStatus(t *testing.T) {
	outbox := setupOutboxTestDB(t)
	ctx := context.Background()

	id, err := outbox.Enqueue(ctx, nil, "topic", "key1", []byte(`{"a":1}`))
	if err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	if err := outbox.MarkPublished(ctx, id); err != nil {
		t.Fatalf("MarkPublished failed: %v", err)
	}

	// FetchPending would no longer return this row; GetByID should still
	// find it, since it's not status-filtered.
	got, err := outbox.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if got.Status != "published" {
		t.Errorf("Status = %q, want %q", got.Status, "published")
	}
	assertJSONEqual(t, got.Payload, []byte(`{"a":1}`))
}

// assertJSONEqual compares two JSON byte strings semantically, not
// byte-for-byte — Postgres's JSONB column normalizes whitespace on
// storage (e.g. adds a space after ':'), so a raw string comparison
// against what was originally inserted is not reliable.
func assertJSONEqual(t *testing.T, got, want []byte) {
	t.Helper()

	var gotVal, wantVal any
	if err := json.Unmarshal(got, &gotVal); err != nil {
		t.Fatalf("got is not valid JSON: %v (%q)", err, got)
	}
	if err := json.Unmarshal(want, &wantVal); err != nil {
		t.Fatalf("want is not valid JSON: %v (%q)", err, want)
	}

	if !reflect.DeepEqual(gotVal, wantVal) {
		t.Errorf("Payload = %s, want %s", got, want)
	}
}
