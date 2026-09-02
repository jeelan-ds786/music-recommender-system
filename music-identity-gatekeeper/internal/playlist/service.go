package playlist

import (
	"context"

	"github.com/google/uuid"

	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/logger"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/reqid"
)

// Service is the domain contract consumed by the HTTP layer.
type Service interface {
	Create(ctx context.Context, userID uuid.UUID, req CreateRequest) (*PlaylistResponse, error)
	List(ctx context.Context, userID uuid.UUID) ([]PlaylistResponse, error)
	Get(ctx context.Context, userID, playlistID uuid.UUID) (*PlaylistDetailResponse, error)
	Patch(ctx context.Context, userID, playlistID uuid.UUID, req PatchRequest) (*PlaylistResponse, error)
	Delete(ctx context.Context, userID, playlistID uuid.UUID) error
	AddSong(ctx context.Context, userID, playlistID, songID uuid.UUID) error
	RemoveSong(ctx context.Context, userID, playlistID, songID uuid.UUID) error
}

type service struct {
	repo Repository
	log  *logger.Logger
}

// NewService leaves no emitter param — E1-SS-15 adds Kafka event
// publishing later the same way preference.NewService grew one.
func NewService(repo Repository, log *logger.Logger) Service {
	return &service{
		repo: repo,
		log:  log,
	}
}

func toPlaylistResponse(p *Playlist) PlaylistResponse {
	return PlaylistResponse{
		ID:          p.ID,
		Name:        p.Name,
		Description: p.Description,
		IsPublic:    p.IsPublic,
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	}
}

func (s *service) Create(
	ctx context.Context,
	userID uuid.UUID,
	req CreateRequest,
) (*PlaylistResponse, error) {

	rid, _ := reqid.FromContext(ctx)

	s.log.Debug(rid, "Starting Create for user_id=%s", userID)

	p, err := s.repo.Create(ctx, userID, req.Name, req.Description, req.IsPublic)
	if err != nil {
		s.log.Error(rid, "Ending Create for user_id=%s (failed: %v)", userID, err)
		return nil, err
	}

	s.log.Info(rid, "Ending Create for user_id=%s (playlist_id=%s created)", userID, p.ID)

	resp := toPlaylistResponse(p)
	return &resp, nil
}

func (s *service) List(
	ctx context.Context,
	userID uuid.UUID,
) ([]PlaylistResponse, error) {

	rid, _ := reqid.FromContext(ctx)

	s.log.Debug(rid, "Starting List for user_id=%s", userID)

	playlists, err := s.repo.ListByUser(ctx, userID)
	if err != nil {
		s.log.Error(rid, "Ending List for user_id=%s (failed: %v)", userID, err)
		return nil, err
	}

	resp := make([]PlaylistResponse, 0, len(playlists))
	for _, p := range playlists {
		resp = append(resp, toPlaylistResponse(&p))
	}

	s.log.Debug(rid, "Ending List for user_id=%s", userID)

	return resp, nil
}

func (s *service) Get(
	ctx context.Context,
	userID uuid.UUID,
	playlistID uuid.UUID,
) (*PlaylistDetailResponse, error) {

	rid, _ := reqid.FromContext(ctx)

	s.log.Debug(rid, "Starting Get for user_id=%s playlist_id=%s", userID, playlistID)

	p, err := s.repo.GetByID(ctx, playlistID, userID)
	if err != nil {
		s.log.Error(rid, "Ending Get for user_id=%s playlist_id=%s (failed: %v)", userID, playlistID, err)
		return nil, err
	}

	songs, err := s.repo.ListSongs(ctx, playlistID, userID)
	if err != nil {
		s.log.Error(rid, "Ending Get for user_id=%s playlist_id=%s (failed: %v)", userID, playlistID, err)
		return nil, err
	}

	items := make([]PlaylistSongItem, 0, len(songs))
	for _, song := range songs {
		items = append(items, PlaylistSongItem(song))
	}

	s.log.Debug(rid, "Ending Get for user_id=%s playlist_id=%s", userID, playlistID)

	return &PlaylistDetailResponse{
		PlaylistResponse: toPlaylistResponse(p),
		Songs:            items,
	}, nil
}

func (s *service) Patch(
	ctx context.Context,
	userID uuid.UUID,
	playlistID uuid.UUID,
	req PatchRequest,
) (*PlaylistResponse, error) {

	rid, _ := reqid.FromContext(ctx)

	s.log.Debug(rid, "Starting Patch for user_id=%s playlist_id=%s", userID, playlistID)

	p, err := s.repo.Update(ctx, playlistID, userID, PatchFields(req))
	if err != nil {
		s.log.Error(rid, "Ending Patch for user_id=%s playlist_id=%s (failed: %v)", userID, playlistID, err)
		return nil, err
	}

	s.log.Info(rid, "Ending Patch for user_id=%s playlist_id=%s (playlist updated)", userID, playlistID)

	resp := toPlaylistResponse(p)
	return &resp, nil
}

func (s *service) Delete(
	ctx context.Context,
	userID uuid.UUID,
	playlistID uuid.UUID,
) error {

	rid, _ := reqid.FromContext(ctx)

	s.log.Debug(rid, "Starting Delete for user_id=%s playlist_id=%s", userID, playlistID)

	if err := s.repo.Delete(ctx, playlistID, userID); err != nil {
		s.log.Error(rid, "Ending Delete for user_id=%s playlist_id=%s (failed: %v)", userID, playlistID, err)
		return err
	}

	s.log.Info(rid, "Ending Delete for user_id=%s playlist_id=%s (playlist deleted)", userID, playlistID)

	return nil
}

func (s *service) AddSong(
	ctx context.Context,
	userID uuid.UUID,
	playlistID uuid.UUID,
	songID uuid.UUID,
) error {

	rid, _ := reqid.FromContext(ctx)

	s.log.Debug(rid, "Starting AddSong for user_id=%s playlist_id=%s song_id=%s", userID, playlistID, songID)

	if err := s.repo.AddSong(ctx, playlistID, userID, songID); err != nil {
		s.log.Error(rid, "Ending AddSong for user_id=%s playlist_id=%s song_id=%s (failed: %v)", userID, playlistID, songID, err)
		return err
	}

	s.log.Info(rid, "Ending AddSong for user_id=%s playlist_id=%s song_id=%s (song added)", userID, playlistID, songID)

	return nil
}

func (s *service) RemoveSong(
	ctx context.Context,
	userID uuid.UUID,
	playlistID uuid.UUID,
	songID uuid.UUID,
) error {

	rid, _ := reqid.FromContext(ctx)

	s.log.Debug(rid, "Starting RemoveSong for user_id=%s playlist_id=%s song_id=%s", userID, playlistID, songID)

	if err := s.repo.RemoveSong(ctx, playlistID, userID, songID); err != nil {
		s.log.Error(rid, "Ending RemoveSong for user_id=%s playlist_id=%s song_id=%s (failed: %v)", userID, playlistID, songID, err)
		return err
	}

	s.log.Info(rid, "Ending RemoveSong for user_id=%s playlist_id=%s song_id=%s (song removed)", userID, playlistID, songID)

	return nil
}
