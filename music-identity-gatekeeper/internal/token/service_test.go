package token

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/logger"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/refresh"
)

type staticTierProvider struct{ tier string }

func (p staticTierProvider) GetTier(context.Context, uuid.UUID) (string, error) {
	return p.tier, nil
}

type memoryRefreshRepository struct {
	mu     sync.Mutex
	tokens map[string]refresh.RefreshToken
}

type recordingBlacklist struct {
	jti string
	ttl time.Duration
	err error
}

func (b *recordingBlacklist) Blacklist(_ context.Context, jti string, ttl time.Duration) error {
	b.jti = jti
	b.ttl = ttl
	return b.err
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
	service := NewService(NewJWTService("test-secret"), repository, staticTierProvider{tier: "free"}, logger.New(logger.LevelNone))
	initialPair, err := service.IssueTokenPair(ctx, uuid.New())
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

func TestLogoutRevokesAccessAndRefreshTokens(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	repository := newMemoryRefreshRepository()
	blacklist := &recordingBlacklist{}
	service := NewService(
		NewJWTService("test-secret"),
		repository,
		staticTierProvider{tier: "free"},
		logger.New(logger.LevelNone),
		blacklist,
	)
	pair, err := service.IssueTokenPair(ctx, userID)
	if err != nil {
		t.Fatalf("IssueTokenPair() error = %v", err)
	}
	claims, err := service.jwtService.ParseAccessToken(pair.AccessToken)
	if err != nil {
		t.Fatalf("ParseAccessToken() error = %v", err)
	}
	remainingLifetime := time.Until(claims.ExpiresAt.Time)

	if err := service.Logout(ctx, userID, pair.RefreshToken, claims.ID, claims.ExpiresAt.Time); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
	if blacklist.jti != claims.ID {
		t.Fatalf("blacklisted jti = %q, want %q", blacklist.jti, claims.ID)
	}
	if blacklist.ttl <= 0 || blacklist.ttl > remainingLifetime {
		t.Fatalf("blacklist ttl = %v, want within (0, %v]", blacklist.ttl, remainingLifetime)
	}
	if _, err := service.RefreshAccessToken(ctx, pair.RefreshToken); err == nil {
		t.Fatal("revoked refresh token was accepted")
	}
}

func TestLogoutFailsWhenBlacklistIsUnavailable(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	service := NewService(
		NewJWTService("test-secret"),
		newMemoryRefreshRepository(),
		staticTierProvider{tier: "free"},
		logger.New(logger.LevelNone),
		&recordingBlacklist{err: errors.New("redis unavailable")},
	)
	pair, err := service.IssueTokenPair(ctx, userID)
	if err != nil {
		t.Fatalf("IssueTokenPair() error = %v", err)
	}

	err = service.Logout(ctx, userID, pair.RefreshToken, "jti", time.Now().Add(time.Minute))
	if !errors.Is(err, ErrRevocationUnavailable) {
		t.Fatalf("Logout() error = %v, want ErrRevocationUnavailable", err)
	}
}

func TestLogoutRejectsAnotherUsersRefreshToken(t *testing.T) {
	ctx := context.Background()
	repository := newMemoryRefreshRepository()
	blacklist := &recordingBlacklist{}
	service := NewService(
		NewJWTService("test-secret"),
		repository,
		staticTierProvider{tier: "free"},
		logger.New(logger.LevelNone),
		blacklist,
	)
	pair, err := service.IssueTokenPair(ctx, uuid.New())
	if err != nil {
		t.Fatalf("IssueTokenPair() error = %v", err)
	}

	err = service.Logout(ctx, uuid.New(), pair.RefreshToken, "jti", time.Now().Add(time.Minute))
	if !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("Logout() error = %v, want ErrInvalidRefreshToken", err)
	}
	if blacklist.jti != "" {
		t.Fatal("access token was blacklisted before refresh-token ownership validation")
	}
}
