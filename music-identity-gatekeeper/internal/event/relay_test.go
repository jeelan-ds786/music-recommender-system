package event

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/logger"
)

func TestRelay_PublishesPendingEvents(t *testing.T) {
	outbox := setupOutboxTestDB(t) // Reuse setup from outbox_test.go
	publisher := &FakePublisher{}
	log := logger.New(logger.LevelNone)

	relay := NewRelay(outbox, publisher, log, WithPollInterval(50*time.Millisecond))

	ctx := context.Background()
	// payload is a JSONB column — must be valid JSON.
	if _, err := outbox.Enqueue(ctx, nil, "topic1", "key1", []byte(`{"n":"payload1"}`)); err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}
	if _, err := outbox.Enqueue(ctx, nil, "topic2", "key2", []byte(`{"n":"payload2"}`)); err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	relay.Start()

	// Wait for relay to process
	time.Sleep(200 * time.Millisecond)

	if err := relay.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}

	if len(publisher.Published) != 2 {
		t.Fatalf("expected 2 published messages, got %d", len(publisher.Published))
	}

	// Verify events are marked published
	pending, _ := outbox.FetchPending(ctx, 10)
	if len(pending) != 0 {
		t.Errorf("expected 0 pending events, got %d", len(pending))
	}
}

func TestRelay_RetriesOnFailureAndMarksFailed(t *testing.T) {
	outbox := setupOutboxTestDB(t)
	publisher := &FakePublisher{Err: errors.New("simulated error")}
	log := logger.New(logger.LevelNone)

	// Set max attempts to 3 to fail fast
	relay := NewRelay(outbox, publisher, log, WithPollInterval(50*time.Millisecond), WithMaxAttempts(3))

	ctx := context.Background()
	if _, err := outbox.Enqueue(ctx, nil, "topic1", "key1", []byte(`{"n":"payload1"}`)); err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	relay.Start()

	// Wait for relay to exhaust attempts (3 * 50ms)
	time.Sleep(300 * time.Millisecond)

	if err := relay.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}

	if len(publisher.Published) != 0 {
		t.Fatalf("expected 0 published messages due to error, got %d", len(publisher.Published))
	}

	// Verify event is NOT pending (it should be 'failed')
	pending, _ := outbox.FetchPending(ctx, 10)
	if len(pending) != 0 {
		t.Errorf("expected 0 pending events (should be failed), got %d", len(pending))
	}
}
