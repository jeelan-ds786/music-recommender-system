package consumer

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestConsumerCommitsOnlyAfterHandlingAndIdempotency(t *testing.T) {
	events := make([]string, 0, 3)
	reader := &fakeReader{messages: []Message{{EventID: "event-1"}}, events: &events}
	handler := &fakeHandler{events: &events}
	store := &fakeStore{events: &events}
	consumer := New(reader, handler, store)

	if err := consumer.process(context.Background(), reader.messages[0]); err != nil {
		t.Fatal(err)
	}

	want := []string{"handle", "complete", "commit"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
}

func TestConsumerDoesNotCommitHandlerFailure(t *testing.T) {
	events := make([]string, 0, 2)
	message := Message{EventID: "event-1"}
	reader := &fakeReader{events: &events}
	handler := &fakeHandler{events: &events, err: errors.New("handler failed")}
	consumer := New(reader, handler, &fakeStore{events: &events})

	if err := consumer.process(context.Background(), message); err == nil {
		t.Fatal("process() error = nil, want handler failure")
	}
	if !reflect.DeepEqual(events, []string{"handle"}) {
		t.Fatalf("events = %v, want only handle", events)
	}
}

func TestConsumerCommitsDuplicateWithoutHandling(t *testing.T) {
	events := make([]string, 0, 1)
	message := Message{EventID: "event-1"}
	reader := &fakeReader{events: &events}
	handler := &fakeHandler{events: &events}
	store := &fakeStore{events: &events, claim: ClaimProcessed}
	consumer := New(reader, handler, store)

	if err := consumer.process(context.Background(), message); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(events, []string{"commit"}) {
		t.Fatalf("events = %v, want only commit", events)
	}
}

func TestConsumerWaitsForConcurrentDuplicateBeforeCommit(t *testing.T) {
	events := make([]string, 0, 1)
	message := Message{EventID: "event-1"}
	reader := &fakeReader{events: &events}
	store := &fakeStore{events: &events, claims: []ClaimState{ClaimBusy, ClaimProcessed}}
	consumer := New(reader, &fakeHandler{events: &events}, store)

	if err := consumer.process(context.Background(), message); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(events, []string{"commit"}) {
		t.Fatalf("events = %v, want commit only after other worker completes", events)
	}
	if store.claimCalls != 2 {
		t.Fatalf("claim calls = %d, want 2", store.claimCalls)
	}
}

func TestRunGroupDrainsInFlightMessage(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	events := make([]string, 0, 3)
	var eventsMu sync.Mutex
	reader := &blockingReader{
		message: Message{EventID: "event-1"},
		events:  &events,
		mu:      &eventsMu,
	}
	handler := &blockingHandler{started: started, release: release, events: &events, mu: &eventsMu}
	worker := New(reader, handler, &lockingStore{events: &events, mu: &eventsMu})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- RunGroup(ctx, time.Second, worker) }()

	<-started
	cancel()
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}

	eventsMu.Lock()
	defer eventsMu.Unlock()
	if !reflect.DeepEqual(events, []string{"handle", "complete", "commit"}) {
		t.Fatalf("events = %v, want drained handle, complete, commit", events)
	}
}

type fakeReader struct {
	messages []Message
	events   *[]string
}

func (r *fakeReader) Fetch(context.Context) (Message, error) {
	if len(r.messages) == 0 {
		return Message{}, context.Canceled
	}
	message := r.messages[0]
	r.messages = r.messages[1:]
	return message, nil
}

func (r *fakeReader) Commit(context.Context, Message) error {
	*r.events = append(*r.events, "commit")
	return nil
}

func (*fakeReader) Close() error { return nil }

type fakeHandler struct {
	events *[]string
	err    error
}

func (*fakeHandler) Name() string { return "test-handler" }

func (h *fakeHandler) Handle(context.Context, Message) error {
	*h.events = append(*h.events, "handle")
	return h.err
}

type fakeStore struct {
	events     *[]string
	claim      ClaimState
	claims     []ClaimState
	claimCalls int
}

type blockingReader struct {
	message Message
	fetched bool
	events  *[]string
	mu      *sync.Mutex
}

func (r *blockingReader) Fetch(ctx context.Context) (Message, error) {
	if !r.fetched {
		r.fetched = true
		return r.message, nil
	}
	<-ctx.Done()
	return Message{}, ctx.Err()
}

func (r *blockingReader) Commit(context.Context, Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	*r.events = append(*r.events, "commit")
	return nil
}

func (*blockingReader) Close() error { return nil }

type blockingHandler struct {
	started chan struct{}
	release chan struct{}
	events  *[]string
	mu      *sync.Mutex
}

func (*blockingHandler) Name() string { return "blocking-handler" }

func (h *blockingHandler) Handle(ctx context.Context, _ Message) error {
	close(h.started)
	select {
	case <-h.release:
		h.mu.Lock()
		defer h.mu.Unlock()
		*h.events = append(*h.events, "handle")
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type lockingStore struct {
	events *[]string
	mu     *sync.Mutex
}

func (*lockingStore) Claim(context.Context, string, string) (ClaimState, error) {
	return ClaimAcquired, nil
}

func (s *lockingStore) Complete(context.Context, string, string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	*s.events = append(*s.events, "complete")
	return nil
}

func (*lockingStore) Release(context.Context, string, string) error { return nil }

func (s *fakeStore) Claim(context.Context, string, string) (ClaimState, error) {
	s.claimCalls++
	if len(s.claims) > 0 {
		claim := s.claims[0]
		s.claims = s.claims[1:]
		return claim, nil
	}
	return s.claim, nil
}

func (s *fakeStore) Complete(context.Context, string, string) error {
	*s.events = append(*s.events, "complete")
	return nil
}

func (*fakeStore) Release(context.Context, string, string) error { return nil }
