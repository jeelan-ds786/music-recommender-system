package event

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/event/eventpb"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/logger"
)

func TestEmitter_EmitUserRegistered_EnqueuesCorrectPayload(t *testing.T) {
	outbox := setupOutboxTestDB(t)
	log := logger.New(logger.LevelNone)
	// A failing publisher keeps this test focused on payload construction:
	// the row stays 'pending', so the existing FetchPending-based
	// assertions below are unaffected by the direct-publish step.
	emitter := NewEmitter(outbox, log, &FakePublisher{Err: errors.New("simulated: broker unavailable")})

	userID := uuid.New().String()
	email := "test@example.com"
	ctx := context.Background()

	err := emitter.EmitUserRegistered(ctx, nil, userID, email)
	if err != nil {
		t.Fatalf("EmitUserRegistered failed: %v", err)
	}

	events, _ := outbox.FetchPending(ctx, 1)
	if len(events) != 1 {
		t.Fatalf("expected 1 pending event, got %d", len(events))
	}

	e := events[0]
	if e.Topic != TopicUserRegistered {
		t.Errorf("Topic = %q, want %q", e.Topic, TopicUserRegistered)
	}
	if e.Key != userID {
		t.Errorf("Key = %q, want %q", e.Key, userID)
	}

	var msg eventpb.UserRegistered
	if err := protojson.Unmarshal(e.Payload, &msg); err != nil {
		t.Fatalf("failed to unmarshal JSON payload: %v", err)
	}

	if msg.Email != email {
		t.Errorf("Email = %q, want %q", msg.Email, email)
	}
	if msg.Metadata.UserId != userID {
		t.Errorf("UserId = %q, want %q", msg.Metadata.UserId, userID)
	}
	if msg.Metadata.EventType != TopicUserRegistered {
		t.Errorf("EventType = %q, want %q", msg.Metadata.EventType, TopicUserRegistered)
	}
	if msg.Metadata.EventId == "" {
		t.Error("EventId is empty")
	}
}

func TestEmitter_EmitUserPreferenceUpdated_EnqueuesCorrectPayload(t *testing.T) {
	outbox := setupOutboxTestDB(t)
	log := logger.New(logger.LevelNone)
	emitter := NewEmitter(outbox, log, &FakePublisher{Err: errors.New("simulated: broker unavailable")})

	userID := uuid.New().String()
	ctx := context.Background()

	err := emitter.EmitUserPreferenceUpdated(ctx, nil, userID)
	if err != nil {
		t.Fatalf("EmitUserPreferenceUpdated failed: %v", err)
	}

	events, _ := outbox.FetchPending(ctx, 1)
	if len(events) != 1 {
		t.Fatalf("expected 1 pending event, got %d", len(events))
	}

	e := events[0]
	if e.Topic != TopicUserPreferenceUpdated {
		t.Errorf("Topic = %q, want %q", e.Topic, TopicUserPreferenceUpdated)
	}
	if e.Key != userID {
		t.Errorf("Key = %q, want %q", e.Key, userID)
	}

	var msg eventpb.UserPreferenceUpdated
	if err := protojson.Unmarshal(e.Payload, &msg); err != nil {
		t.Fatalf("failed to unmarshal JSON payload: %v", err)
	}

	if msg.Metadata.UserId != userID {
		t.Errorf("UserId = %q, want %q", msg.Metadata.UserId, userID)
	}
	if msg.Metadata.EventType != TopicUserPreferenceUpdated {
		t.Errorf("EventType = %q, want %q", msg.Metadata.EventType, TopicUserPreferenceUpdated)
	}
}

func TestEmitter_EmitUserRegistered_SuccessfulDirectPublishMarksPublished(t *testing.T) {
	outbox := setupOutboxTestDB(t)
	log := logger.New(logger.LevelNone)
	fake := &FakePublisher{}
	emitter := NewEmitter(outbox, log, fake)

	userID := uuid.New().String()
	ctx := context.Background()

	if err := emitter.EmitUserRegistered(ctx, nil, userID, "test@example.com"); err != nil {
		t.Fatalf("EmitUserRegistered failed: %v", err)
	}

	if len(fake.Published) != 1 {
		t.Fatalf("expected 1 direct publish attempt, got %d", len(fake.Published))
	}

	// The row should no longer be pending — it was published directly, not
	// left for the relay.
	pending, _ := outbox.FetchPending(ctx, 10)
	if len(pending) != 0 {
		t.Fatalf("expected 0 pending events after a successful direct publish, got %d", len(pending))
	}
}

func TestEmitter_EmitUserRegistered_FailedDirectPublishLeavesPendingForRelay(t *testing.T) {
	outbox := setupOutboxTestDB(t)
	log := logger.New(logger.LevelNone)
	fake := &FakePublisher{Err: errors.New("simulated: broker unavailable")}
	emitter := NewEmitter(outbox, log, fake)

	userID := uuid.New().String()
	ctx := context.Background()

	// A failed direct publish must not surface as an error to the caller —
	// the event is still safely durable in the outbox.
	if err := emitter.EmitUserRegistered(ctx, nil, userID, "test@example.com"); err != nil {
		t.Fatalf("EmitUserRegistered returned an error on publish failure, want nil: %v", err)
	}

	pending, err := outbox.FetchPending(ctx, 10)
	if err != nil {
		t.Fatalf("FetchPending failed: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending event left for the relay, got %d", len(pending))
	}
	if pending[0].Attempts != 1 {
		t.Errorf("expected attempts = 1 after the failed direct attempt, got %d", pending[0].Attempts)
	}
}
