package playlist

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/db"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/user"
)

// newTestUser connects to Postgres via DB_URL, creates a throwaway user
// (cleaned up via t.Cleanup), and returns everything a mutation test needs.
// Mirrors preference's helper of the same name.
func newTestUser(t *testing.T) (context.Context, *pgxpool.Pool, Repository, uuid.UUID) {
	t.Helper()

	_ = godotenv.Load("../../.env")

	dsn := os.Getenv("DB_URL")
	if dsn == "" {
		t.Skip("DB_URL not set, skipping integration test")
	}

	ctx := context.Background()

	pool, err := db.NewPostgresPool(ctx, dsn, nil)
	if err != nil {
		t.Fatalf("failed to connect to postgres: %v", err)
	}
	t.Cleanup(pool.Close)

	userRepo := user.NewRepository(pool)
	testUser := &user.User{
		ID:             uuid.New(),
		Email:          "playlist-test-" + uuid.NewString() + "@example.com",
		HashedPassword: "test-hash",
		AuthProvider:   "local",
	}

	if err := userRepo.CreateUser(ctx, testUser); err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
	t.Cleanup(func() {
		// ON DELETE CASCADE on users removes playlists/playlist_songs rows
		// for this user too.
		if _, err := pool.Exec(ctx, "DELETE FROM users WHERE id = $1", testUser.ID); err != nil {
			t.Logf("cleanup: failed to delete test user: %v", err)
		}
	})

	return ctx, pool, NewRepository(pool), testUser.ID
}

// newTestPlaylist creates a throwaway user and a playlist owned by them.
func newTestPlaylist(t *testing.T) (context.Context, Repository, uuid.UUID, uuid.UUID) {
	t.Helper()

	ctx, _, repo, userID := newTestUser(t)

	p, err := repo.Create(ctx, userID, "My Playlist", nil, false)
	if err != nil {
		t.Fatalf("failed to create test playlist: %v", err)
	}

	return ctx, repo, userID, p.ID
}

func TestPostgresRepository_CreateAndGet(t *testing.T) {
	ctx, _, repo, userID := newTestUser(t)

	desc := "a description"
	p, err := repo.Create(ctx, userID, "Road Trip", &desc, true)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	got, err := repo.GetByID(ctx, p.ID, userID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if got.Name != "Road Trip" {
		t.Errorf("expected name %q, got %q", "Road Trip", got.Name)
	}
	if got.Description == nil || *got.Description != desc {
		t.Errorf("expected description %q, got %v", desc, got.Description)
	}
	if !got.IsPublic {
		t.Errorf("expected IsPublic true, got false")
	}
}

func TestPostgresRepository_GetByID_NotFoundForOtherUser(t *testing.T) {
	ctx, repo, _, playlistID := newTestPlaylist(t)
	_, _, _, otherUserID := newTestUser(t)

	_, err := repo.GetByID(ctx, playlistID, otherUserID)
	if !errors.Is(err, ErrPlaylistNotFound) {
		t.Errorf("expected ErrPlaylistNotFound, got %v", err)
	}
}

func TestPostgresRepository_GetByID_Unknown(t *testing.T) {
	ctx, _, repo, userID := newTestUser(t)

	_, err := repo.GetByID(ctx, uuid.New(), userID)
	if !errors.Is(err, ErrPlaylistNotFound) {
		t.Errorf("expected ErrPlaylistNotFound, got %v", err)
	}
}

func TestPostgresRepository_ListByUser(t *testing.T) {
	ctx, _, repo, userID := newTestUser(t)

	if _, err := repo.Create(ctx, userID, "Playlist A", nil, false); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if _, err := repo.Create(ctx, userID, "Playlist B", nil, false); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	_, _, _, otherUserID := newTestUser(t)
	if _, err := repo.Create(ctx, otherUserID, "Not Mine", nil, false); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	got, err := repo.ListByUser(ctx, userID)
	if err != nil {
		t.Fatalf("ListByUser failed: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 playlists, got %d", len(got))
	}
}

func TestPostgresRepository_Update_PatchPreservesOmittedFields(t *testing.T) {
	ctx, repo, userID, playlistID := newTestPlaylist(t)

	newName := "Renamed"
	got, err := repo.Update(ctx, playlistID, userID, PatchFields{Name: &newName})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	if got.Name != newName {
		t.Errorf("expected name %q, got %q", newName, got.Name)
	}
	if got.IsPublic {
		t.Errorf("expected IsPublic to be preserved as false, got true")
	}
}

func TestPostgresRepository_Update_NotFoundForOtherUser(t *testing.T) {
	ctx, repo, _, playlistID := newTestPlaylist(t)
	_, _, _, otherUserID := newTestUser(t)

	newName := "Hijacked"
	_, err := repo.Update(ctx, playlistID, otherUserID, PatchFields{Name: &newName})
	if !errors.Is(err, ErrPlaylistNotFound) {
		t.Errorf("expected ErrPlaylistNotFound, got %v", err)
	}
}

func TestPostgresRepository_Delete(t *testing.T) {
	ctx, repo, userID, playlistID := newTestPlaylist(t)

	if err := repo.Delete(ctx, playlistID, userID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err := repo.GetByID(ctx, playlistID, userID)
	if !errors.Is(err, ErrPlaylistNotFound) {
		t.Errorf("expected ErrPlaylistNotFound after delete, got %v", err)
	}
}

func TestPostgresRepository_Delete_NotFoundForOtherUser(t *testing.T) {
	ctx, repo, _, playlistID := newTestPlaylist(t)
	_, _, _, otherUserID := newTestUser(t)

	err := repo.Delete(ctx, playlistID, otherUserID)
	if !errors.Is(err, ErrPlaylistNotFound) {
		t.Errorf("expected ErrPlaylistNotFound, got %v", err)
	}
}

func TestPostgresRepository_AddSong_IdempotentAndAssignsPositions(t *testing.T) {
	ctx, repo, userID, playlistID := newTestPlaylist(t)
	songA := uuid.New()
	songB := uuid.New()

	if err := repo.AddSong(ctx, playlistID, userID, songA); err != nil {
		t.Fatalf("AddSong(A) failed: %v", err)
	}
	if err := repo.AddSong(ctx, playlistID, userID, songA); err != nil {
		t.Fatalf("AddSong(A) idempotent retry failed: %v", err)
	}
	if err := repo.AddSong(ctx, playlistID, userID, songB); err != nil {
		t.Fatalf("AddSong(B) failed: %v", err)
	}

	songs, err := repo.ListSongs(ctx, playlistID, userID)
	if err != nil {
		t.Fatalf("ListSongs failed: %v", err)
	}
	if len(songs) != 2 {
		t.Fatalf("expected exactly 2 songs, got %d", len(songs))
	}
	if songs[0].SongID != songA || songs[0].Position != 0 {
		t.Errorf("expected song A at position 0, got %+v", songs[0])
	}
	if songs[1].SongID != songB || songs[1].Position != 1 {
		t.Errorf("expected song B at position 1, got %+v", songs[1])
	}
}

func TestPostgresRepository_AddSong_PlaylistNotFoundForOtherUser(t *testing.T) {
	ctx, repo, _, playlistID := newTestPlaylist(t)
	_, _, _, otherUserID := newTestUser(t)

	err := repo.AddSong(ctx, playlistID, otherUserID, uuid.New())
	if !errors.Is(err, ErrPlaylistNotFound) {
		t.Errorf("expected ErrPlaylistNotFound, got %v", err)
	}
}

// TestPostgresRepository_AddSong_ConcurrentDuplicatesLeaveOneRow mirrors
// preference.TestPostgresRepository_LikeSong_ConcurrentDuplicatesLeaveOneRow:
// the FOR UPDATE row lock on the parent playlist must serialize concurrent
// AddSong calls for the same song so they don't race on "next position"
// and leave duplicate/gapped rows.
func TestPostgresRepository_AddSong_ConcurrentDuplicatesLeaveOneRow(t *testing.T) {
	ctx, repo, userID, playlistID := newTestPlaylist(t)
	songID := uuid.New()

	var wg sync.WaitGroup
	errs := make([]error, 5)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = repo.AddSong(ctx, playlistID, userID, songID)
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("concurrent AddSong[%d] failed: %v", i, err)
		}
	}

	songs, err := repo.ListSongs(ctx, playlistID, userID)
	if err != nil {
		t.Fatalf("ListSongs failed: %v", err)
	}
	if len(songs) != 1 {
		t.Fatalf("expected exactly 1 row after concurrent adds, got %d", len(songs))
	}
}

func TestPostgresRepository_RemoveSong_MissingReturnsNotFound(t *testing.T) {
	ctx, repo, userID, playlistID := newTestPlaylist(t)

	err := repo.RemoveSong(ctx, playlistID, userID, uuid.New())
	if !errors.Is(err, ErrSongNotInPlaylist) {
		t.Errorf("expected ErrSongNotInPlaylist, got %v", err)
	}
}

func TestPostgresRepository_RemoveSong_NotFoundForOtherUser(t *testing.T) {
	ctx, repo, userID, playlistID := newTestPlaylist(t)
	songID := uuid.New()

	if err := repo.AddSong(ctx, playlistID, userID, songID); err != nil {
		t.Fatalf("AddSong failed: %v", err)
	}

	_, _, _, otherUserID := newTestUser(t)
	err := repo.RemoveSong(ctx, playlistID, otherUserID, songID)
	if !errors.Is(err, ErrSongNotInPlaylist) {
		t.Errorf("expected ErrSongNotInPlaylist, got %v", err)
	}
}

func TestPostgresRepository_RemoveSong_RemovesRow(t *testing.T) {
	ctx, repo, userID, playlistID := newTestPlaylist(t)
	songID := uuid.New()

	if err := repo.AddSong(ctx, playlistID, userID, songID); err != nil {
		t.Fatalf("AddSong failed: %v", err)
	}
	if err := repo.RemoveSong(ctx, playlistID, userID, songID); err != nil {
		t.Fatalf("RemoveSong failed: %v", err)
	}

	songs, err := repo.ListSongs(ctx, playlistID, userID)
	if err != nil {
		t.Fatalf("ListSongs failed: %v", err)
	}
	if len(songs) != 0 {
		t.Fatalf("expected 0 songs after remove, got %d", len(songs))
	}

	err = repo.RemoveSong(ctx, playlistID, userID, songID)
	if !errors.Is(err, ErrSongNotInPlaylist) {
		t.Errorf("expected ErrSongNotInPlaylist on second remove, got %v", err)
	}
}
