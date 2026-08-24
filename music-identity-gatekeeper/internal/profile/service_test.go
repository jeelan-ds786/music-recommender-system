package profile

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/token"
)

type memoryRepository struct {
	tier         string
	upgradeCalls int
}

func (r *memoryRepository) GetTier(context.Context, uuid.UUID) (string, error) {
	return r.tier, nil
}

func (r *memoryRepository) Upgrade(_ context.Context, _ uuid.UUID, tier string) error {
	r.upgradeCalls++
	r.tier = tier
	return nil
}

type claimTokenIssuer struct {
	repository *memoryRepository
	jwtService *token.JWTService
	calls      int
}

func (i *claimTokenIssuer) IssueTokenPair(
	_ context.Context,
	userID uuid.UUID,
) (*token.TokenPair, error) {
	i.calls++
	accessToken, err := i.jwtService.GenerateAccessToken(userID.String(), i.repository.tier)
	if err != nil {
		return nil, err
	}
	return &token.TokenPair{AccessToken: accessToken, RefreshToken: "reissued"}, nil
}

func TestUpgradeRejectsInvalidTier(t *testing.T) {
	repository := &memoryRepository{tier: TierFree}
	issuer := &claimTokenIssuer{repository: repository, jwtService: token.NewJWTService("secret")}
	service := NewService(repository, issuer)

	pair, err := service.Upgrade(context.Background(), uuid.New(), "admin")
	if !errors.Is(err, ErrInvalidTier) {
		t.Fatalf("Upgrade() error = %v, want ErrInvalidTier", err)
	}
	if pair != nil || repository.upgradeCalls != 0 || issuer.calls != 0 {
		t.Fatal("invalid tier changed state or issued tokens")
	}
}

func TestUpgradeReissuesTokenWithNewTier(t *testing.T) {
	userID := uuid.New()
	repository := &memoryRepository{tier: TierFree}
	jwtService := token.NewJWTService("secret")
	issuer := &claimTokenIssuer{repository: repository, jwtService: jwtService}
	service := NewService(repository, issuer)

	pair, err := service.Upgrade(context.Background(), userID, TierPremium)
	if err != nil {
		t.Fatalf("Upgrade() error = %v", err)
	}
	claims, err := jwtService.ParseAccessToken(pair.AccessToken)
	if err != nil {
		t.Fatalf("ParseAccessToken() error = %v", err)
	}
	if claims.UserID != userID.String() || claims.Tier != TierPremium {
		t.Fatalf("claims = %#v", claims)
	}
	if repository.upgradeCalls != 1 || issuer.calls != 1 {
		t.Fatalf("upgrade calls = %d, issuer calls = %d", repository.upgradeCalls, issuer.calls)
	}
}
