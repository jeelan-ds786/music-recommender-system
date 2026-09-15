package event

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

type fakeOutbox struct {
	pending     []Message
	published   []uuid.UUID
	failures    []uuid.UUID
	fetchLimit  int
	maxAttempts int
}

func (o *fakeOutbox) MarkPublished(_ context.Context, id uuid.UUID) error {
	o.published = append(o.published, id)
	return nil
}

func (o *fakeOutbox) RecordFailure(_ context.Context, id uuid.UUID, _ string) error {
	o.failures = append(o.failures, id)
	return nil
}

func (o *fakeOutbox) FetchPending(_ context.Context, limit, maxAttempts int) ([]Message, error) {
	o.fetchLimit = limit
	o.maxAttempts = maxAttempts
	return o.pending, nil
}

func TestDirectFailureLeavesMessageForRelay(t *testing.T) {
	message := Message{ID: uuid.New(), Topic: PlaybackTopic, Key: uuid.NewString()}
	outbox := &fakeOutbox{}
	directPublisher := &FakePublisher{Err: errors.New("broker unavailable")}

	NewDirect(outbox, directPublisher).Publish(context.Background(), message)

	if len(outbox.failures) != 1 || outbox.failures[0] != message.ID {
		t.Fatalf("direct failures = %v, want message %s retained", outbox.failures, message.ID)
	}
	if len(outbox.published) != 0 {
		t.Fatalf("direct publish marked failed message published: %v", outbox.published)
	}

	outbox.pending = []Message{message}
	relayPublisher := &FakePublisher{}
	if err := NewRelay(outbox, relayPublisher, 25, 5).RunOnce(context.Background()); err != nil {
		t.Fatalf("relay RunOnce() error = %v", err)
	}
	if len(relayPublisher.Published) != 1 || relayPublisher.Published[0].ID != message.ID {
		t.Fatalf("relay published = %+v, want retained message", relayPublisher.Published)
	}
	if len(outbox.published) != 1 || outbox.published[0] != message.ID {
		t.Fatalf("relay marked published = %v, want %s", outbox.published, message.ID)
	}
	if outbox.fetchLimit != 25 || outbox.maxAttempts != 5 {
		t.Fatalf("relay bounds = limit %d attempts %d", outbox.fetchLimit, outbox.maxAttempts)
	}
}

func TestDirectSuccessDoesNotNeedRelay(t *testing.T) {
	message := Message{ID: uuid.New(), Topic: PlaybackTopic, Key: uuid.NewString()}
	outbox := &fakeOutbox{}
	publisher := &FakePublisher{}

	NewDirect(outbox, publisher).Publish(context.Background(), message)

	if len(publisher.Published) != 1 {
		t.Fatalf("direct published %d messages, want 1", len(publisher.Published))
	}
	if len(outbox.published) != 1 || len(outbox.failures) != 0 {
		t.Fatalf("published = %v failures = %v", outbox.published, outbox.failures)
	}
}
