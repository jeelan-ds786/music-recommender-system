package oauth

import (
	"context"
	"errors"
	"testing"
	"time"
)

type memoryStateStore struct {
	states map[string]time.Time
	now    time.Time
}

func newMemoryStateStore() *memoryStateStore {
	return &memoryStateStore{states: make(map[string]time.Time), now: time.Now()}
}

func (s *memoryStateStore) Save(_ context.Context, stateHash string, ttl time.Duration) error {
	s.states[stateHash] = s.now.Add(ttl)
	return nil
}

func (s *memoryStateStore) Consume(_ context.Context, stateHash string) (bool, error) {
	expiresAt, ok := s.states[stateHash]
	delete(s.states, stateHash)
	return ok && s.now.Before(expiresAt), nil
}

func TestStateManagerRejectsMissingInvalidReusedAndExpiredStates(t *testing.T) {
	ctx := context.Background()
	store := newMemoryStateStore()
	manager := NewStateManager(store)

	if err := manager.Validate(ctx, ""); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("missing state error = %v", err)
	}
	if err := manager.Validate(ctx, "unknown"); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("unknown state error = %v", err)
	}

	state, err := manager.Create(ctx)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := manager.Validate(ctx, state); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if err := manager.Validate(ctx, state); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("reused state error = %v", err)
	}

	expiredState, err := manager.Create(ctx)
	if err != nil {
		t.Fatalf("Create() expired state error = %v", err)
	}
	store.now = store.now.Add(StateTTL)
	if err := manager.Validate(ctx, expiredState); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("expired state error = %v", err)
	}
}

func TestStateManagerStoresOnlyStateHash(t *testing.T) {
	ctx := context.Background()
	store := newMemoryStateStore()
	manager := NewStateManager(store)
	state, err := manager.Create(ctx)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, exists := store.states[state]; exists {
		t.Fatal("raw OAuth state was persisted")
	}
	if _, exists := store.states[hashState(state)]; !exists {
		t.Fatal("hashed OAuth state was not persisted")
	}
}
