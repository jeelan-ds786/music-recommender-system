package profile

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/logger"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/preference"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/reqid"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/token"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/user"
)

const (
	TierFree    = "free"
	TierPremium = "premium"
	TierFamily  = "family"
	TierStudent = "student"
	TierDuo     = "duo"
)

var ErrInvalidTier = errors.New("invalid subscription tier")

type TokenIssuer interface {
	IssueTokenPair(ctx context.Context, userID uuid.UUID) (*token.TokenPair, error)
}

type TierService struct {
	repository  TierRepository
	tokenIssuer TokenIssuer
}

func NewService(repository TierRepository, tokenIssuer TokenIssuer) *TierService {
	return &TierService{repository: repository, tokenIssuer: tokenIssuer}
}

func (s *TierService) GetTier(ctx context.Context, userID uuid.UUID) (string, error) {
	return s.repository.GetTier(ctx, userID)
}

func (s *TierService) Upgrade(
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

type Service interface {
	GetMe(ctx context.Context, userID uuid.UUID) (*MeResponse, error)
	PatchMe(ctx context.Context, userID uuid.UUID, req PatchMeRequest) (*MeResponse, error)
}

type service struct {
	profileRepo ProfileRepository
	prefRepo    preference.Repository
	userRepo    user.Repository
	log         *logger.Logger
}

func NewProfileService(
	profileRepo ProfileRepository,
	prefRepo preference.Repository,
	userRepo user.Repository,
	log *logger.Logger,
) Service {
	return &service{
		profileRepo: profileRepo,
		prefRepo:    prefRepo,
		userRepo:    userRepo,
		log:         log,
	}
}

func (s *service) GetMe(
	ctx context.Context,
	userID uuid.UUID,
) (*MeResponse, error) {

	rid, _ := reqid.FromContext(ctx)

	s.log.Debug(rid, "Starting GetMe for user_id=%s", userID)

	u, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		s.log.Error(rid, "Ending GetMe for user_id=%s (user lookup failed: %v)", userID, err)
		return nil, err
	}

	p, pref, err := s.loadProfileAndPreferences(ctx, userID)
	if err != nil {
		s.log.Error(rid, "Ending GetMe for user_id=%s (profile/preference load failed: %v)", userID, err)
		return nil, err
	}

	s.log.Debug(rid, "Ending GetMe for user_id=%s", userID)

	return buildMeResponse(u, p, pref), nil
}

func (s *service) PatchMe(
	ctx context.Context,
	userID uuid.UUID,
	req PatchMeRequest,
) (*MeResponse, error) {

	rid, _ := reqid.FromContext(ctx)

	s.log.Debug(rid, "Starting PatchMe for user_id=%s", userID)

	u, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		s.log.Error(rid, "Ending PatchMe for user_id=%s (user lookup failed: %v)", userID, err)
		return nil, err
	}

	s.log.Debug(rid, "checking profile row exists for user_id=%s", userID)

	// A PATCH from a user who never called GET /me first must still
	// succeed, so make sure the rows exist before updating.
	if _, err := s.profileRepo.GetByUserID(ctx, userID); errors.Is(err, ErrProfileNotFound) {
		s.log.Info(rid, "no profile row found for user_id=%s, creating one before patch", userID)

		if err := s.profileRepo.EnsureExists(ctx, userID); err != nil {
			s.log.Error(rid, "Ending PatchMe for user_id=%s (EnsureExists failed: %v)", userID, err)
			return nil, err
		}
	} else if err != nil {
		s.log.Error(rid, "Ending PatchMe for user_id=%s (profile lookup failed: %v)", userID, err)
		return nil, err
	}

	p, err := s.profileRepo.Update(ctx, userID, PatchFields(req))
	if err != nil {
		s.log.Error(rid, "Ending PatchMe for user_id=%s (write failed: %v)", userID, err)
		return nil, err
	}

	s.log.Info(rid, "write completed for user_id=%s", userID)

	pref, err := s.prefRepo.GetByUserID(ctx, userID)
	if err != nil {
		s.log.Error(rid, "Ending PatchMe for user_id=%s (preference lookup failed: %v)", userID, err)
		return nil, err
	}

	s.log.Debug(rid, "Ending PatchMe for user_id=%s", userID)

	return buildMeResponse(u, p, pref), nil
}

// loadProfileAndPreferences reads both rows directly first, since they
// exist for almost every request once a user has been touched once. Only
// on a genuine first-ever /me call (either row missing) does it pay the
// EnsureExists transaction, then re-reads both.
func (s *service) loadProfileAndPreferences(
	ctx context.Context,
	userID uuid.UUID,
) (*Profile, *preference.Preference, error) {

	rid, _ := reqid.FromContext(ctx)

	s.log.Debug(rid, "checking profile and preference rows for user_id=%s", userID)

	p, err := s.profileRepo.GetByUserID(ctx, userID)
	profileMissing := errors.Is(err, ErrProfileNotFound)
	if err != nil && !profileMissing {
		return nil, nil, err
	}

	pref, err := s.prefRepo.GetByUserID(ctx, userID)
	prefMissing := errors.Is(err, preference.ErrPreferenceNotFound)
	if err != nil && !prefMissing {
		return nil, nil, err
	}

	if !profileMissing && !prefMissing {
		s.log.Debug(rid, "found existing profile and preference rows for user_id=%s", userID)
		return p, pref, nil
	}

	s.log.Info(rid, "no data found for user_id=%s, creating profile/preference rows", userID)

	if err := s.profileRepo.EnsureExists(ctx, userID); err != nil {
		return nil, nil, err
	}

	p, err = s.profileRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, nil, err
	}

	pref, err = s.prefRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, nil, err
	}

	return p, pref, nil
}

func buildMeResponse(u *user.User, p *Profile, pref *preference.Preference) *MeResponse {
	return &MeResponse{
		Account: AccountFields{
			ID:        u.ID,
			Email:     u.Email,
			CreatedAt: u.CreatedAt,
		},
		Profile: ProfileFields{
			DisplayName: p.DisplayName,
			AvatarURL:   p.AvatarURL,
			Country:     p.Country,
			Language:    p.Language,
			BirthYear:   p.BirthYear,
			UpdatedAt:   p.UpdatedAt,
		},
		Tier: p.SubscriptionTier,
		Preferences: PreferenceFields{
			LikedSongIDs:      pref.LikedSongIDs,
			FollowedArtistIDs: pref.FollowedArtistIDs,
			GenreSeeds:        pref.GenreSeeds,
			LanguagePrefs:     pref.LanguagePrefs,
		},
	}
}
