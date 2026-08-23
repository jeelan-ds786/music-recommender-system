package token

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/refresh"
)

type memoryRefreshRepository struct {
	mu     sync.Mutex
	tokens map[string]refresh.RefreshToken
}

func newMemoryRefreshRepository() *memoryRefreshRepository {
	return &memoryRefreshRepository{tokens: make(map[string]refresh.RefreshToken)}
}

func (r *memoryRefreshRepository) CreateRefreshToken(_ context.Context, token *refresh.RefreshToken) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tokens[token.TokenHash] = *token
	return nil
}

func (r *memoryRefreshRepository) GetByHash(_ context.Context, tokenHash string) (*refresh.RefreshToken, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	token, ok := r.tokens[tokenHash]
	if !ok {
		return nil, errors.New("refresh token not found")
	}
	return &token, nil
}

func (r *memoryRefreshRepository) Revoke(_ context.Context, tokenHash string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	token, ok := r.tokens[tokenHash]
	if !ok {
		return errors.New("refresh token not found")
	}
	token.Revoked = true
	r.tokens[tokenHash] = token
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
		return errors.New("refresh token is unavailable")
	}
	oldToken.Revoked = true
	r.tokens[oldTokenHash] = oldToken
	r.tokens[newToken.TokenHash] = *newToken
	return nil
}

func TestRefreshTokenRotationRejectsReuse(t *testing.T) {
	ctx := context.Background()
	repository := newMemoryRefreshRepository()
	service := NewService(NewJWTService("test-secret"), repository)
	initialPair, err := service.IssueTokenPair(ctx, uuid.New(), "local")
	if err != nil {
		t.Fatalf("issue initial token pair: %v", err)
	}

	rotatedPair, err := service.RefreshAccessToken(ctx, initialPair.RefreshToken)
	if err != nil {
		t.Fatalf("rotate refresh token: %v", err)
	}
	if rotatedPair.RefreshToken == initialPair.RefreshToken {
		t.Fatal("rotation returned the old refresh token")
	}
	if _, err := service.jwtService.ParseAccessToken(rotatedPair.AccessToken); err != nil {
		t.Fatalf("rotated access token is invalid: %v", err)
	}

	if _, err := service.RefreshAccessToken(ctx, initialPair.RefreshToken); err == nil {
		t.Fatal("reusing the revoked refresh token succeeded")
	}
}
