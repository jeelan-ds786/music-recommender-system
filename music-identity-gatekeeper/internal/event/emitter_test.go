package event

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
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

func TestEmitter_EmitPlaylistUpdated_PublishesCorrectPayload(t *testing.T) {
	outbox := setupOutboxTestDB(t)
	fake := &FakePublisher{}
	emitter := NewEmitter(outbox, logger.New(logger.LevelNone), fake)
	userID := uuid.New().String()
	playlistID := uuid.New().String()

	if err := emitter.EmitPlaylistUpdated(context.Background(), nil, userID, playlistID, PlaylistOperationSongAdded); err != nil {
		t.Fatalf("EmitPlaylistUpdated failed: %v", err)
	}
	if len(fake.Published) != 1 {
		t.Fatalf("published %d messages, want 1", len(fake.Published))
	}

	published := fake.Published[0]
	if published.Topic != TopicUserPlaylistUpdated || published.Key != playlistID {
		t.Errorf("published topic=%q key=%q, want topic=%q key=%q", published.Topic, published.Key, TopicUserPlaylistUpdated, playlistID)
	}

	var msg eventpb.PlaylistUpdated
	if err := protojson.Unmarshal(published.Value, &msg); err != nil {
		t.Fatalf("failed to unmarshal JSON payload: %v", err)
	}
	if msg.GetMetadata().GetUserId() != userID || msg.GetMetadata().GetEventType() != TopicUserPlaylistUpdated {
		t.Errorf("metadata = %+v", msg.GetMetadata())
	}
	if msg.GetPlaylistId() != playlistID || msg.GetOperation() != PlaylistOperationSongAdded {
		t.Errorf("playlist_id=%q operation=%q, want playlist_id=%q operation=%q", msg.GetPlaylistId(), msg.GetOperation(), playlistID, PlaylistOperationSongAdded)
	}
}

func TestEmitter_EmitPlaylistUpdated_DeliversToKafka(t *testing.T) {
	brokersEnv := os.Getenv("KAFKA_BROKERS")
	if brokersEnv == "" {
		t.Skip("KAFKA_BROKERS not set, skipping integration test")
	}

	outbox := setupOutboxTestDB(t)
	brokers := strings.Split(brokersEnv, ",")
	publisher := NewKafkaPublisher(brokers, logger.New(logger.LevelNone))
	t.Cleanup(func() { _ = publisher.Close() })

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:   brokers,
		Topic:     TopicUserPlaylistUpdated,
		Partition: 0,
		MinBytes:  1,
		MaxBytes:  10e6,
	})
	t.Cleanup(func() { _ = reader.Close() })
	if err := reader.SetOffset(kafka.FirstOffset); err != nil {
		t.Fatalf("SetOffset failed: %v", err)
	}

	userID := uuid.New().String()
	playlistID := uuid.New().String()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	emitter := NewEmitter(outbox, logger.New(logger.LevelNone), publisher)
	if err := emitter.EmitPlaylistUpdated(ctx, nil, userID, playlistID, PlaylistOperationCreated); err != nil {
		t.Fatalf("EmitPlaylistUpdated failed: %v", err)
	}

	var (
		message kafka.Message
		err     error
	)
	for string(message.Key) != playlistID {
		message, err = reader.ReadMessage(ctx)
		if err != nil {
			t.Fatalf("ReadMessage failed: %v", err)
		}
	}

	var eventMessage eventpb.PlaylistUpdated
	if err := protojson.Unmarshal(message.Value, &eventMessage); err != nil {
		t.Fatalf("failed to unmarshal delivered payload: %v", err)
	}
	if eventMessage.GetMetadata().GetUserId() != userID || eventMessage.GetPlaylistId() != playlistID || eventMessage.GetOperation() != PlaylistOperationCreated {
		t.Errorf("delivered event = %+v", &eventMessage)
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
