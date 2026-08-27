package event

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/event/eventpb"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/logger"
)

func TestFakePublisher_RecordsCalls(t *testing.T) {
	fake := &FakePublisher{}

	if err := fake.Publish(context.Background(), TopicUserRegistered, "user-1", []byte("payload")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(fake.Published) != 1 {
		t.Fatalf("expected 1 published message, got %d", len(fake.Published))
	}

	got := fake.Published[0]
	if got.Topic != TopicUserRegistered || got.Key != "user-1" || string(got.Value) != "payload" {
		t.Errorf("unexpected recorded message: %+v", got)
	}

	if err := fake.Close(); err != nil {
		t.Errorf("unexpected error from Close: %v", err)
	}
}

func TestFakePublisher_PropagatesConfiguredError(t *testing.T) {
	wantErr := errors.New("broker unavailable")
	fake := &FakePublisher{Err: wantErr}

	err := fake.Publish(context.Background(), TopicUserRegistered, "user-1", []byte("payload"))
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}

	if len(fake.Published) != 0 {
		t.Errorf("expected nothing recorded on failure, got %v", fake.Published)
	}
}

func TestNoopPublisher_DiscardsSilently(t *testing.T) {
	var p Publisher = NoopPublisher{}

	if err := p.Publish(context.Background(), TopicUserPreferenceUpdated, "user-1", []byte("payload")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := p.Close(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestEventpb_RoundTrip proves the generated protobuf messages actually
// marshal/unmarshal correctly, not just that protoc-gen-go produced
// something that compiles.
func TestEventpb_RoundTrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)

	original := &eventpb.UserRegistered{
		Metadata: &eventpb.EventMetadata{
			EventId:       "11111111-1111-1111-1111-111111111111",
			EventType:     "user.registered",
			SchemaVersion: 1,
			OccurredAt:    timestamppb.New(now),
			UserId:        "22222222-2222-2222-2222-222222222222",
		},
		Email: "listener@example.com",
	}

	raw, err := proto.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	decoded := &eventpb.UserRegistered{}
	if err := proto.Unmarshal(raw, decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.GetEmail() != original.Email {
		t.Errorf("Email = %q, want %q", decoded.GetEmail(), original.Email)
	}
	if decoded.GetMetadata().GetEventId() != original.Metadata.EventId {
		t.Errorf("EventId = %q, want %q", decoded.GetMetadata().GetEventId(), original.Metadata.EventId)
	}
	if decoded.GetMetadata().GetSchemaVersion() != original.Metadata.SchemaVersion {
		t.Errorf("SchemaVersion = %d, want %d", decoded.GetMetadata().GetSchemaVersion(), original.Metadata.SchemaVersion)
	}
	if !decoded.GetMetadata().GetOccurredAt().AsTime().Equal(now) {
		t.Errorf("OccurredAt = %v, want %v", decoded.GetMetadata().GetOccurredAt().AsTime(), now)
	}
}

// TestKafkaPublisher_ConnectsToLocalBroker is an integration test: it
// needs a real Kafka broker reachable via KAFKA_BROKERS (same env var the
// service will use). It skips itself when KAFKA_BROKERS isn't set, so
// `go test ./...` still passes without Kafka running — same self-skip
// pattern as the DB_URL-gated repository tests.
func TestKafkaPublisher_ConnectsToLocalBroker(t *testing.T) {
	brokersEnv := os.Getenv("KAFKA_BROKERS")
	if brokersEnv == "" {
		t.Skip("KAFKA_BROKERS not set, skipping integration test")
	}

	// Respects LOG_LEVEL (e.g. LOG_LEVEL=debug) so the Starting/Ending
	// Publish log lines are actually visible when running this manually,
	// same as the server's own logging.
	brokers := strings.Split(brokersEnv, ",")
	publisher := NewKafkaPublisher(brokers, logger.New(logger.ParseLevel(os.Getenv("LOG_LEVEL"))))
	t.Cleanup(func() {
		if err := publisher.Close(); err != nil {
			t.Logf("cleanup: failed to close publisher: %v", err)
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := publisher.Publish(ctx, TopicUserRegistered, "integration-test", []byte("hello")); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}
}
