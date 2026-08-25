package preference

import (
	"context"

	"github.com/google/uuid"
)

// Service is the domain contract consumed by the HTTP and gRPC layers.
// Mutation methods (like/unlike, follow/unfollow, onboarding) are added
// in E1-SS-05; this ticket only defines the read contract.
type Service interface {
	GetPreferences(ctx context.Context, userID uuid.UUID) (*PreferenceResponse, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{
		repo: repo,
	}
}

func (s *service) GetPreferences(
	ctx context.Context,
	userID uuid.UUID,
) (*PreferenceResponse, error) {

	p, err := s.repo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &PreferenceResponse{
		LikedSongIDs:      p.LikedSongIDs,
		FollowedArtistIDs: p.FollowedArtistIDs,
		GenreSeeds:        p.GenreSeeds,
		LanguagePrefs:     p.LanguagePrefs,
	}, nil
}
