package oauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"
)

const StateTTL = 10 * time.Minute

var ErrInvalidState = errors.New("invalid oauth state")

type StateStore interface {
	Save(ctx context.Context, stateHash string, ttl time.Duration) error
	Consume(ctx context.Context, stateHash string) (bool, error)
}

type StateManager struct {
	store StateStore
}

func NewStateManager(store StateStore) *StateManager {
	return &StateManager{store: store}
}

func (m *StateManager) Create(ctx context.Context) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	state := hex.EncodeToString(raw)
	if err := m.store.Save(ctx, hashState(state), StateTTL); err != nil {
		return "", err
	}
	return state, nil
}

func (m *StateManager) Validate(ctx context.Context, state string) error {
	if state == "" {
		return ErrInvalidState
	}
	valid, err := m.store.Consume(ctx, hashState(state))
	if err != nil {
		return err
	}
	if !valid {
		return ErrInvalidState
	}
	return nil
}

func hashState(state string) string {
	hash := sha256.Sum256([]byte(state))
	return hex.EncodeToString(hash[:])
}
