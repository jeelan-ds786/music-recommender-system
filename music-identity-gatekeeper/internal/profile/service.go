package profile

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/token"
)

const (
	TierFree    = "free"
	TierPremium = "premium"
	TierFamily  = "family"
	TierStudent = "student"
	TierDuo     = "duo"
)

var ErrInvalidTier = errors.New("invalid subscription tier")

type Repository interface {
	GetTier(ctx context.Context, userID uuid.UUID) (string, error)
	Upgrade(ctx context.Context, userID uuid.UUID, tier string) error
}

type TokenIssuer interface {
	IssueTokenPair(ctx context.Context, userID uuid.UUID) (*token.TokenPair, error)
}

type Service struct {
	repository  Repository
	tokenIssuer TokenIssuer
}

func NewService(repository Repository, tokenIssuer TokenIssuer) *Service {
	return &Service{repository: repository, tokenIssuer: tokenIssuer}
}

func (s *Service) GetTier(ctx context.Context, userID uuid.UUID) (string, error) {
	return s.repository.GetTier(ctx, userID)
}

func (s *Service) Upgrade(
	ctx context.Context,
	userID uuid.UUID,
	tier string,
) (*token.TokenPair, error) {
	if !IsValidTier(tier) {
		return nil, ErrInvalidTier
	}
	if err := s.repository.Upgrade(ctx, userID, tier); err != nil {
		return nil, err
	}
	return s.tokenIssuer.IssueTokenPair(ctx, userID)
}

func IsValidTier(tier string) bool {
	switch tier {
	case TierFree, TierPremium, TierFamily, TierStudent, TierDuo:
		return true
	default:
		return false
	}
}
