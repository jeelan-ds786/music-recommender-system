package preference

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/joho/godotenv"

	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/db"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/user"
)

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
