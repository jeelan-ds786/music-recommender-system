package preference

import (
	"context"

	"github.com/google/uuid"

	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/logger"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/reqid"
)

const defaultLikedSongsLimit = 20
const maxLikedSongsLimit = 100

// Service is the domain contract consumed by the HTTP and gRPC layers.
type Service interface {
	GetPreferences(ctx context.Context, userID uuid.UUID) (*PreferenceResponse, error)
	Onboard(ctx context.Context, userID uuid.UUID, req OnboardingRequest) error
	LikeSong(ctx context.Context, userID, songID uuid.UUID) error
	UnlikeSong(ctx context.Context, userID, songID uuid.UUID) error
	ListLikedSongs(ctx context.Context, userID uuid.UUID, cursor string, limit int) (*LikedSongsPage, error)
	FollowArtist(ctx context.Context, userID, artistID uuid.UUID) error
	UnfollowArtist(ctx context.Context, userID, artistID uuid.UUID) error
}

type service struct {
	repo Repository
	log  *logger.Logger
}

func NewService(repo Repository, log *logger.Logger) Service {
	return &service{
		repo: repo,
		log:  log,
	}
}

func (s *service) GetPreferences(
	ctx context.Context,
	userID uuid.UUID,
) (*PreferenceResponse, error) {

	rid, _ := reqid.FromContext(ctx)

	s.log.Debug(rid, "Starting GetPreferences for user_id=%s", userID)

	p, err := s.repo.GetByUserID(ctx, userID)
	if err != nil {
		s.log.Error(rid, "Ending GetPreferences for user_id=%s (failed: %v)", userID, err)
		return nil, err
	}

	s.log.Debug(rid, "Ending GetPreferences for user_id=%s", userID)

	return &PreferenceResponse{
		LikedSongIDs:      p.LikedSongIDs,
		FollowedArtistIDs: p.FollowedArtistIDs,
		GenreSeeds:        p.GenreSeeds,
		LanguagePrefs:     p.LanguagePrefs,
	}, nil
}

func (s *service) Onboard(
	ctx context.Context,
	userID uuid.UUID,
	req OnboardingRequest,
) error {

	rid, _ := reqid.FromContext(ctx)

	s.log.Debug(rid, "Starting Onboard for user_id=%s", userID)

	err := s.repo.CompleteOnboarding(ctx, userID, OnboardingFields{
		GenreSeeds:        req.GenreSeeds,
		LanguagePrefs:     req.LanguagePrefs,
		FollowedArtistIDs: req.FollowedArtistIDs,
	})
	if err != nil {
		s.log.Error(rid, "Ending Onboard for user_id=%s (failed: %v)", userID, err)
		return err
	}

	s.log.Info(rid, "Ending Onboard for user_id=%s", userID)

	return nil
}

func (s *service) LikeSong(
	ctx context.Context,
	userID uuid.UUID,
	songID uuid.UUID,
) error {

	rid, _ := reqid.FromContext(ctx)

	s.log.Debug(rid, "Starting LikeSong for user_id=%s song_id=%s", userID, songID)

	if err := s.repo.LikeSong(ctx, userID, songID); err != nil {
		s.log.Error(rid, "Ending LikeSong for user_id=%s song_id=%s (failed: %v)", userID, songID, err)
		return err
	}

	s.log.Info(rid, "Ending LikeSong for user_id=%s song_id=%s (song liked)", userID, songID)

	return nil
}

func (s *service) UnlikeSong(
	ctx context.Context,
	userID uuid.UUID,
	songID uuid.UUID,
) error {

	rid, _ := reqid.FromContext(ctx)

	s.log.Debug(rid, "Starting UnlikeSong for user_id=%s song_id=%s", userID, songID)

	if err := s.repo.UnlikeSong(ctx, userID, songID); err != nil {
		s.log.Error(rid, "Ending UnlikeSong for user_id=%s song_id=%s (failed: %v)", userID, songID, err)
		return err
	}

	s.log.Info(rid, "Ending UnlikeSong for user_id=%s song_id=%s (song unliked)", userID, songID)

	return nil
}

func (s *service) ListLikedSongs(
	ctx context.Context,
	userID uuid.UUID,
	cursor string,
	limit int,
) (*LikedSongsPage, error) {

	rid, _ := reqid.FromContext(ctx)

	s.log.Debug(rid, "Starting ListLikedSongs for user_id=%s cursor=%q limit=%d", userID, cursor, limit)

	var c *Cursor
	if cursor != "" {
		decoded, err := DecodeCursor(cursor)
		if err != nil {
			s.log.Error(rid, "Ending ListLikedSongs for user_id=%s (invalid cursor %q: %v)", userID, cursor, err)
			return nil, err
		}
		c = decoded
		s.log.Debug(rid, "decoded cursor for user_id=%s: created_at=%s id=%s", userID, c.CreatedAt, c.ID)
	}

	if limit <= 0 {
		limit = defaultLikedSongsLimit
	}
	if limit > maxLikedSongsLimit {
		limit = maxLikedSongsLimit
	}

	items, next, err := s.repo.ListLikedSongs(ctx, userID, c, limit)
	if err != nil {
		s.log.Error(rid, "Ending ListLikedSongs for user_id=%s (repo query failed: %v)", userID, err)
		return nil, err
	}

	page := &LikedSongsPage{
		Items: make([]LikedSongItem, 0, len(items)),
	}
	for _, item := range items {
		page.Items = append(page.Items, LikedSongItem{
			SongID:  item.SongID,
			LikedAt: item.CreatedAt,
		})
	}
	if next != nil {
		encoded := EncodeCursor(*next)
		page.NextCursor = &encoded
	}

	s.log.Debug(
		rid,
		"Cursor calculation done for user_id=%s: %d item(s) returned, next_cursor_set=%t",
		userID,
		len(page.Items),
		page.NextCursor != nil,
	)

	s.log.Debug(rid, "Ending ListLikedSongs for user_id=%s", userID)

	return page, nil
}

func (s *service) FollowArtist(
	ctx context.Context,
	userID uuid.UUID,
	artistID uuid.UUID,
) error {

	rid, _ := reqid.FromContext(ctx)

	s.log.Debug(rid, "Starting FollowArtist for user_id=%s artist_id=%s", userID, artistID)

	if err := s.repo.FollowArtist(ctx, userID, artistID); err != nil {
		s.log.Error(rid, "Ending FollowArtist for user_id=%s artist_id=%s (failed: %v)", userID, artistID, err)
		return err
	}

	s.log.Info(rid, "Ending FollowArtist for user_id=%s artist_id=%s (artist followed)", userID, artistID)

	return nil
}

func (s *service) UnfollowArtist(
	ctx context.Context,
	userID uuid.UUID,
	artistID uuid.UUID,
) error {

	rid, _ := reqid.FromContext(ctx)

	s.log.Debug(rid, "Starting UnfollowArtist for user_id=%s artist_id=%s", userID, artistID)

	if err := s.repo.UnfollowArtist(ctx, userID, artistID); err != nil {
		s.log.Error(rid, "Ending UnfollowArtist for user_id=%s artist_id=%s (failed: %v)", userID, artistID, err)
		return err
	}

	s.log.Info(rid, "Ending UnfollowArtist for user_id=%s artist_id=%s (artist unfollowed)", userID, artistID)

	return nil
}
