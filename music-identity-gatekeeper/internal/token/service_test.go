package token

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/refresh"
)

type memoryRefreshRepository struct {
	mu     sync.Mutex
	tokens map[string]*refresh.RefreshToken
}

func newMemoryRefreshRepository(tokens ...*refresh.RefreshToken) *memoryRefreshRepository {
	repository := &memoryRefreshRepository{tokens: make(map[string]*refresh.RefreshToken)}
	for _, storedToken := range tokens {
		copy := *storedToken
		repository.tokens[storedToken.TokenHash] = &copy
	}
	return repository
}

func (r *memoryRefreshRepository) CreateRefreshToken(_ context.Context, newToken *refresh.RefreshToken) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	copy := *newToken
	r.tokens[newToken.TokenHash] = &copy
	return nil
}

func (r *memoryRefreshRepository) GetByHash(_ context.Context, tokenHash string) (*refresh.RefreshToken, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	storedToken, ok := r.tokens[tokenHash]
	if !ok {
		return nil, errors.New("refresh token not found")
	}
	copy := *storedToken
	return &copy, nil
}

func (r *memoryRefreshRepository) Revoke(_ context.Context, tokenHash string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	storedToken, ok := r.tokens[tokenHash]
	if !ok {
		return errors.New("refresh token not found")
	}
	storedToken.Revoked = true
	return nil
}

func (r *memoryRefreshRepository) Rotate(
	_ context.Context,
	oldTokenHash string,
	newToken *refresh.RefreshToken,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	oldToken, ok := r.tokens[oldTokenHash]
	if !ok || oldToken.Revoked {
		return errors.New("refresh token unavailable")
	}
	oldToken.Revoked = true
	copy := *newToken
	r.tokens[newToken.TokenHash] = &copy
	return nil
}

func TestRefreshAccessTokenRotatesAndRejectsReuse(t *testing.T) {
	const rawToken = "original-refresh-token"
	userID := uuid.New()
	repository := newMemoryRefreshRepository(&refresh.RefreshToken{
		TokenHash: hashRefreshToken(rawToken),
		UserID:    userID,
		ExpiresAt: time.Now().Add(time.Hour),
	})
	service := NewService(NewJWTService("test-secret"), repository)

	rotatedPair, err := service.RefreshAccessToken(context.Background(), rawToken)
	if err != nil {
		t.Fatalf("first RefreshAccessToken() error = %v", err)
	}
	if rotatedPair.AccessToken == "" || rotatedPair.RefreshToken == "" {
		t.Fatalf("rotated token pair = %#v", rotatedPair)
	}
	if rotatedPair.RefreshToken == rawToken {
		t.Fatal("rotation returned the old refresh token")
	}
	if _, err := repository.GetByHash(context.Background(), hashRefreshToken(rotatedPair.RefreshToken)); err != nil {
		t.Fatalf("new refresh token was not stored: %v", err)
	}

	if _, err := service.RefreshAccessToken(context.Background(), rawToken); err == nil {
		t.Fatal("reusing the revoked refresh token succeeded")
	}
}
