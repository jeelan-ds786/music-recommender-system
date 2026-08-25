package preference

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
// Skips the test when DB_URL isn't set, same as TestPostgresRepository_CreateAndGet.
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
		Email:          "preference-test-" + uuid.NewString() + "@example.com",
		HashedPassword: "test-hash",
		AuthProvider:   "local",
	}

	if err := userRepo.CreateUser(ctx, testUser); err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
	t.Cleanup(func() {
		// ON DELETE CASCADE on users removes preferences/liked_songs/
		// followed_artists rows for this user too.
		if _, err := pool.Exec(ctx, "DELETE FROM users WHERE id = $1", testUser.ID); err != nil {
			t.Logf("cleanup: failed to delete test user: %v", err)
		}
	})

	return ctx, pool, NewRepository(pool), testUser.ID
}

// TestPostgresRepository_CreateAndGet is an integration test: it needs a
// real Postgres reachable via DB_URL (same one the service uses). It skips
// itself when DB_URL isn't set, so `go test ./...` still passes without a
// database available.
func TestPostgresRepository_CreateAndGet(t *testing.T) {
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
	// Registered before the delete-cleanups below so it runs last (t.Cleanup
	// is LIFO) — closing the pool before those deletes run would make them
	// fail silently against a closed pool.
	t.Cleanup(pool.Close)

	userRepo := user.NewRepository(pool)
	testUser := &user.User{
		ID:             uuid.New(),
		Email:          "preference-test-" + uuid.NewString() + "@example.com",
		HashedPassword: "test-hash",
		AuthProvider:   "local",
	}

	if err := userRepo.CreateUser(ctx, testUser); err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(ctx, "DELETE FROM users WHERE id = $1", testUser.ID); err != nil {
			t.Logf("cleanup: failed to delete test user: %v", err)
		}
	})

	repo := NewRepository(pool)

	if err := repo.Create(ctx, testUser.ID); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(ctx, "DELETE FROM preferences WHERE user_id = $1", testUser.ID); err != nil {
			t.Logf("cleanup: failed to delete test preferences: %v", err)
		}
	})

	got, err := repo.GetByUserID(ctx, testUser.ID)
	if err != nil {
		t.Fatalf("GetByUserID failed: %v", err)
	}

	if len(got.LikedSongIDs) != 0 {
		t.Errorf("expected empty LikedSongIDs, got %v", got.LikedSongIDs)
	}
	if len(got.FollowedArtistIDs) != 0 {
		t.Errorf("expected empty FollowedArtistIDs, got %v", got.FollowedArtistIDs)
	}
	if len(got.GenreSeeds) != 0 {
		t.Errorf("expected empty GenreSeeds, got %v", got.GenreSeeds)
	}
	if len(got.LanguagePrefs) != 0 {
		t.Errorf("expected empty LanguagePrefs, got %v", got.LanguagePrefs)
	}

	_, err = repo.GetByUserID(ctx, uuid.New())
	if !errors.Is(err, ErrPreferenceNotFound) {
		t.Errorf("expected ErrPreferenceNotFound for unknown user, got %v", err)
	}
}

func TestPostgresRepository_LikeSong_IdempotentAndDualWrites(t *testing.T) {
	ctx, _, repo, userID := newTestUser(t)
	songID := uuid.New()

	if err := repo.LikeSong(ctx, userID, songID); err != nil {
		t.Fatalf("first LikeSong failed: %v", err)
	}
	if err := repo.LikeSong(ctx, userID, songID); err != nil {
		t.Fatalf("second LikeSong (idempotent retry) failed: %v", err)
	}

	items, _, err := repo.ListLikedSongs(ctx, userID, nil, 10)
	if err != nil {
		t.Fatalf("ListLikedSongs failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected exactly 1 liked_songs row, got %d", len(items))
	}

	pref, err := repo.GetByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("GetByUserID failed: %v", err)
	}
	if len(pref.LikedSongIDs) != 1 || pref.LikedSongIDs[0] != songID {
		t.Errorf("expected preferences.liked_song_ids to contain exactly %v, got %v", songID, pref.LikedSongIDs)
	}
}

func TestPostgresRepository_LikeSong_ConcurrentDuplicatesLeaveOneRow(t *testing.T) {
	ctx, _, repo, userID := newTestUser(t)
	songID := uuid.New()

	var wg sync.WaitGroup
	errs := make([]error, 5)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = repo.LikeSong(ctx, userID, songID)
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("concurrent LikeSong[%d] failed: %v", i, err)
		}
	}

	items, _, err := repo.ListLikedSongs(ctx, userID, nil, 10)
	if err != nil {
		t.Fatalf("ListLikedSongs failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected exactly 1 row after concurrent likes, got %d", len(items))
	}

	pref, err := repo.GetByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("GetByUserID failed: %v", err)
	}
	if len(pref.LikedSongIDs) != 1 {
		t.Errorf("expected exactly 1 entry in liked_song_ids after concurrent likes, got %v", pref.LikedSongIDs)
	}
}

func TestPostgresRepository_UnlikeSong_MissingReturnsNotFound(t *testing.T) {
	ctx, _, repo, userID := newTestUser(t)

	err := repo.UnlikeSong(ctx, userID, uuid.New())
	if !errors.Is(err, ErrLikeNotFound) {
		t.Errorf("expected ErrLikeNotFound, got %v", err)
	}
}

func TestPostgresRepository_UnlikeSong_RemovesFromBothStores(t *testing.T) {
	ctx, _, repo, userID := newTestUser(t)
	songID := uuid.New()

	if err := repo.LikeSong(ctx, userID, songID); err != nil {
		t.Fatalf("LikeSong failed: %v", err)
	}
	if err := repo.UnlikeSong(ctx, userID, songID); err != nil {
		t.Fatalf("UnlikeSong failed: %v", err)
	}

	items, _, err := repo.ListLikedSongs(ctx, userID, nil, 10)
	if err != nil {
		t.Fatalf("ListLikedSongs failed: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 liked_songs rows after unlike, got %d", len(items))
	}

	pref, err := repo.GetByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("GetByUserID failed: %v", err)
	}
	if len(pref.LikedSongIDs) != 0 {
		t.Errorf("expected empty liked_song_ids after unlike, got %v", pref.LikedSongIDs)
	}
}

func TestPostgresRepository_ListLikedSongs_PaginatesWithoutDuplicatesOrGaps(t *testing.T) {
	ctx, _, repo, userID := newTestUser(t)

	const total = 25
	want := make(map[uuid.UUID]bool, total)
	for i := 0; i < total; i++ {
		songID := uuid.New()
		want[songID] = true
		if err := repo.LikeSong(ctx, userID, songID); err != nil {
			t.Fatalf("LikeSong failed: %v", err)
		}
	}

	got := make(map[uuid.UUID]bool, total)
	var cursor *Cursor
	pages := 0
	for {
		items, next, err := repo.ListLikedSongs(ctx, userID, cursor, 7)
		if err != nil {
			t.Fatalf("ListLikedSongs failed: %v", err)
		}
		pages++
		for _, item := range items {
			if got[item.SongID] {
				t.Fatalf("song %v returned on more than one page", item.SongID)
			}
			got[item.SongID] = true
		}
		if next == nil {
			break
		}
		cursor = next

		if pages > total {
			t.Fatal("pagination did not terminate")
		}
	}

	if len(got) != total {
		t.Fatalf("expected %d unique songs across all pages, got %d", total, len(got))
	}
	for songID := range want {
		if !got[songID] {
			t.Errorf("song %v missing from paginated results", songID)
		}
	}
}

func TestPostgresRepository_FollowArtist_IdempotentAndDualWrites(t *testing.T) {
	ctx, _, repo, userID := newTestUser(t)
	artistID := uuid.New()

	if err := repo.FollowArtist(ctx, userID, artistID); err != nil {
		t.Fatalf("first FollowArtist failed: %v", err)
	}
	if err := repo.FollowArtist(ctx, userID, artistID); err != nil {
		t.Fatalf("second FollowArtist (idempotent retry) failed: %v", err)
	}

	pref, err := repo.GetByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("GetByUserID failed: %v", err)
	}
	if len(pref.FollowedArtistIDs) != 1 || pref.FollowedArtistIDs[0] != artistID {
		t.Errorf("expected followed_artist_ids to contain exactly %v, got %v", artistID, pref.FollowedArtistIDs)
	}
}

func TestPostgresRepository_UnfollowArtist_MissingReturnsNotFound(t *testing.T) {
	ctx, _, repo, userID := newTestUser(t)

	err := repo.UnfollowArtist(ctx, userID, uuid.New())
	if !errors.Is(err, ErrFollowNotFound) {
		t.Errorf("expected ErrFollowNotFound, got %v", err)
	}
}

func TestPostgresRepository_CompleteOnboarding_SecondCallRejected(t *testing.T) {
	ctx, _, repo, userID := newTestUser(t)

	fields := OnboardingFields{
		GenreSeeds:    []string{"pop", "rock"},
		LanguagePrefs: []string{"en"},
	}

	if err := repo.CompleteOnboarding(ctx, userID, fields); err != nil {
		t.Fatalf("first CompleteOnboarding failed: %v", err)
	}

	err := repo.CompleteOnboarding(ctx, userID, fields)
	if !errors.Is(err, ErrOnboardingAlreadyCompleted) {
		t.Errorf("expected ErrOnboardingAlreadyCompleted on second call, got %v", err)
	}

	pref, err := repo.GetByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("GetByUserID failed: %v", err)
	}
	if len(pref.GenreSeeds) != 2 {
		t.Errorf("expected 2 genre seeds, got %v", pref.GenreSeeds)
	}
}

func TestPostgresRepository_CompleteOnboarding_FollowsArtistsInBothStores(t *testing.T) {
	ctx, _, repo, userID := newTestUser(t)
	artistID := uuid.New()

	fields := OnboardingFields{
		GenreSeeds:        []string{"pop"},
		LanguagePrefs:     []string{"en"},
		FollowedArtistIDs: []uuid.UUID{artistID},
	}

	if err := repo.CompleteOnboarding(ctx, userID, fields); err != nil {
		t.Fatalf("CompleteOnboarding failed: %v", err)
	}

	pref, err := repo.GetByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("GetByUserID failed: %v", err)
	}
	if len(pref.FollowedArtistIDs) != 1 || pref.FollowedArtistIDs[0] != artistID {
		t.Errorf("expected followed_artist_ids to contain %v, got %v", artistID, pref.FollowedArtistIDs)
	}

	err = repo.UnfollowArtist(ctx, userID, artistID)
	if err != nil {
		t.Fatalf("expected artist followed via onboarding to be unfollowable, got error: %v", err)
	}
}
