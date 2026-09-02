package playlist

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/logger"
)

type fakeRepository struct {
	playlist  *Playlist
	playlists []Playlist
	songs     []PlaylistSong
	err       error
}

func (f *fakeRepository) Create(ctx context.Context, userID uuid.UUID, name string, description *string, isPublic bool) (*Playlist, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.playlist, nil
}

func (f *fakeRepository) GetByID(ctx context.Context, playlistID, userID uuid.UUID) (*Playlist, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.playlist, nil
}

func (f *fakeRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]Playlist, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.playlists, nil
}

func (f *fakeRepository) Update(ctx context.Context, playlistID, userID uuid.UUID, patch PatchFields) (*Playlist, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.playlist, nil
}

func (f *fakeRepository) Delete(ctx context.Context, playlistID, userID uuid.UUID) error {
	return f.err
}

func (f *fakeRepository) AddSong(ctx context.Context, playlistID, userID, songID uuid.UUID) error {
	return f.err
}

func (f *fakeRepository) RemoveSong(ctx context.Context, playlistID, userID, songID uuid.UUID) error {
	return f.err
}

func (f *fakeRepository) ListSongs(ctx context.Context, playlistID, userID uuid.UUID) ([]PlaylistSong, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.songs, nil
}

func TestService_Create_ReturnsMappedResponse(t *testing.T) {
	desc := "roadtrip mix"
	repo := &fakeRepository{
		playlist: &Playlist{
			ID:          uuid.New(),
			Name:        "Road Trip",
			Description: &desc,
			IsPublic:    true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}
	svc := NewService(repo, logger.New(logger.LevelNone))

	got, err := svc.Create(context.Background(), uuid.New(), CreateRequest{Name: "Road Trip", Description: &desc, IsPublic: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "Road Trip" {
		t.Errorf("expected name %q, got %q", "Road Trip", got.Name)
	}
	if got.Description == nil || *got.Description != desc {
		t.Errorf("expected description %q, got %v", desc, got.Description)
	}
}

func TestService_Create_PropagatesRepoError(t *testing.T) {
	repo := &fakeRepository{err: errors.New("db down")}
	svc := NewService(repo, logger.New(logger.LevelNone))

	_, err := svc.Create(context.Background(), uuid.New(), CreateRequest{Name: "Road Trip"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestService_List_ReturnsMappedResponses(t *testing.T) {
	repo := &fakeRepository{
		playlists: []Playlist{
			{ID: uuid.New(), Name: "A"},
			{ID: uuid.New(), Name: "B"},
		},
	}
	svc := NewService(repo, logger.New(logger.LevelNone))

	got, err := svc.List(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 playlists, got %d", len(got))
	}
}

func TestService_Get_CombinesPlaylistAndSongs(t *testing.T) {
	songID := uuid.New()
	repo := &fakeRepository{
		playlist: &Playlist{ID: uuid.New(), Name: "Mix"},
		songs:    []PlaylistSong{{SongID: songID, Position: 0, AddedAt: time.Now()}},
	}
	svc := NewService(repo, logger.New(logger.LevelNone))

	got, err := svc.Get(context.Background(), uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "Mix" {
		t.Errorf("expected name %q, got %q", "Mix", got.Name)
	}
	if len(got.Songs) != 1 || got.Songs[0].SongID != songID {
		t.Fatalf("expected 1 song %v, got %+v", songID, got.Songs)
	}
}

func TestService_Get_PropagatesNotFound(t *testing.T) {
	repo := &fakeRepository{err: ErrPlaylistNotFound}
	svc := NewService(repo, logger.New(logger.LevelNone))

	_, err := svc.Get(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, ErrPlaylistNotFound) {
		t.Fatalf("expected ErrPlaylistNotFound, got %v", err)
	}
}

func TestService_Patch_PropagatesNotFound(t *testing.T) {
	repo := &fakeRepository{err: ErrPlaylistNotFound}
	svc := NewService(repo, logger.New(logger.LevelNone))

	newName := "Renamed"
	_, err := svc.Patch(context.Background(), uuid.New(), uuid.New(), PatchRequest{Name: &newName})
	if !errors.Is(err, ErrPlaylistNotFound) {
		t.Fatalf("expected ErrPlaylistNotFound, got %v", err)
	}
}

func TestService_Delete_PropagatesNotFound(t *testing.T) {
	repo := &fakeRepository{err: ErrPlaylistNotFound}
	svc := NewService(repo, logger.New(logger.LevelNone))

	err := svc.Delete(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, ErrPlaylistNotFound) {
		t.Fatalf("expected ErrPlaylistNotFound, got %v", err)
	}
}

func TestService_AddSong_PropagatesNotFound(t *testing.T) {
	repo := &fakeRepository{err: ErrPlaylistNotFound}
	svc := NewService(repo, logger.New(logger.LevelNone))

	err := svc.AddSong(context.Background(), uuid.New(), uuid.New(), uuid.New())
	if !errors.Is(err, ErrPlaylistNotFound) {
		t.Fatalf("expected ErrPlaylistNotFound, got %v", err)
	}
}

func TestService_RemoveSong_PropagatesNotFound(t *testing.T) {
	repo := &fakeRepository{err: ErrSongNotInPlaylist}
	svc := NewService(repo, logger.New(logger.LevelNone))

	err := svc.RemoveSong(context.Background(), uuid.New(), uuid.New(), uuid.New())
	if !errors.Is(err, ErrSongNotInPlaylist) {
		t.Fatalf("expected ErrSongNotInPlaylist, got %v", err)
	}
}
