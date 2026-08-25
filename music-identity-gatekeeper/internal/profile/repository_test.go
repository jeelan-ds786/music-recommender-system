package profile

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

// newTestUser creates a real user row (listener_profiles/preferences both
// FK to users) and registers cleanup for it and any profile/preference
// rows created against it during the test.
func newTestUser(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()

	ctx := context.Background()
	userRepo := user.NewRepository(pool)

	testUser := &user.User{
		ID:             uuid.New(),
		Email:          "profile-test-" + uuid.NewString() + "@example.com",
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

	return testUser.ID
}

func connectTestDB(t *testing.T) *pgxpool.Pool {
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

	return pool
}

func TestPostgresRepository_EnsureExists_CreatesRowsIdempotently(t *testing.T) {
	pool := connectTestDB(t)
	ctx := context.Background()
	userID := newTestUser(t, pool)

	repo := NewRepository(pool)
	t.Cleanup(func() {
		if _, err := pool.Exec(ctx, "DELETE FROM listener_profiles WHERE user_id = $1", userID); err != nil {
			t.Logf("cleanup: failed to delete listener_profiles row: %v", err)
		}
		if _, err := pool.Exec(ctx, "DELETE FROM preferences WHERE user_id = $1", userID); err != nil {
			t.Logf("cleanup: failed to delete preferences row: %v", err)
		}
	})

	if err := repo.EnsureExists(ctx, userID); err != nil {
		t.Fatalf("first EnsureExists failed: %v", err)
	}
	if err := repo.EnsureExists(ctx, userID); err != nil {
		t.Fatalf("second EnsureExists failed: %v", err)
	}

	var profileCount, prefCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM listener_profiles WHERE user_id = $1", userID).Scan(&profileCount); err != nil {
		t.Fatalf("failed to count listener_profiles rows: %v", err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM preferences WHERE user_id = $1", userID).Scan(&prefCount); err != nil {
		t.Fatalf("failed to count preferences rows: %v", err)
	}

	if profileCount != 1 {
		t.Errorf("expected exactly 1 listener_profiles row, got %d", profileCount)
	}
	if prefCount != 1 {
		t.Errorf("expected exactly 1 preferences row, got %d", prefCount)
	}
}

func TestPostgresRepository_EnsureExists_ConcurrentCallsAreSafe(t *testing.T) {
	pool := connectTestDB(t)
	ctx := context.Background()
	userID := newTestUser(t, pool)

	repo := NewRepository(pool)
	t.Cleanup(func() {
		if _, err := pool.Exec(ctx, "DELETE FROM listener_profiles WHERE user_id = $1", userID); err != nil {
			t.Logf("cleanup: failed to delete listener_profiles row: %v", err)
		}
		if _, err := pool.Exec(ctx, "DELETE FROM preferences WHERE user_id = $1", userID); err != nil {
			t.Logf("cleanup: failed to delete preferences row: %v", err)
		}
	})

	const n = 10
	var wg sync.WaitGroup
	errs := make([]error, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = repo.EnsureExists(ctx, userID)
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d: EnsureExists failed: %v", i, err)
		}
	}

	var profileCount, prefCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM listener_profiles WHERE user_id = $1", userID).Scan(&profileCount); err != nil {
		t.Fatalf("failed to count listener_profiles rows: %v", err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM preferences WHERE user_id = $1", userID).Scan(&prefCount); err != nil {
		t.Fatalf("failed to count preferences rows: %v", err)
	}

	if profileCount != 1 {
		t.Errorf("expected exactly 1 listener_profiles row after concurrent EnsureExists, got %d", profileCount)
	}
	if prefCount != 1 {
		t.Errorf("expected exactly 1 preferences row after concurrent EnsureExists, got %d", prefCount)
	}
}

func TestPostgresRepository_GetByUserID_NotFound(t *testing.T) {
	pool := connectTestDB(t)
	repo := NewRepository(pool)

	_, err := repo.GetByUserID(context.Background(), uuid.New())
	if !errors.Is(err, ErrProfileNotFound) {
		t.Errorf("expected ErrProfileNotFound, got %v", err)
	}
}

func TestPostgresRepository_Update_PartialPatchPreservesOmittedFields(t *testing.T) {
	pool := connectTestDB(t)
	ctx := context.Background()
	userID := newTestUser(t, pool)

	repo := NewRepository(pool)
	t.Cleanup(func() {
		if _, err := pool.Exec(ctx, "DELETE FROM listener_profiles WHERE user_id = $1", userID); err != nil {
			t.Logf("cleanup: failed to delete listener_profiles row: %v", err)
		}
		if _, err := pool.Exec(ctx, "DELETE FROM preferences WHERE user_id = $1", userID); err != nil {
			t.Logf("cleanup: failed to delete preferences row: %v", err)
		}
	})

	if err := repo.EnsureExists(ctx, userID); err != nil {
		t.Fatalf("EnsureExists failed: %v", err)
	}

	country := "US"
	if _, err := repo.Update(ctx, userID, PatchFields{Country: &country}); err != nil {
		t.Fatalf("first Update failed: %v", err)
	}

	name := "New Name"
	updated, err := repo.Update(ctx, userID, PatchFields{DisplayName: &name})
	if err != nil {
		t.Fatalf("second Update failed: %v", err)
	}

	if updated.DisplayName == nil || *updated.DisplayName != name {
		t.Errorf("expected DisplayName %q, got %v", name, updated.DisplayName)
	}
	if updated.Country == nil || *updated.Country != country {
		t.Errorf("expected Country %q to survive the second patch untouched, got %v", country, updated.Country)
	}
	if updated.AvatarURL != nil {
		t.Errorf("expected AvatarURL to remain nil, got %v", updated.AvatarURL)
	}
}

func TestPostgresRepository_Update_MissingRowReturnsNotFound(t *testing.T) {
	pool := connectTestDB(t)
	ctx := context.Background()
	userID := newTestUser(t, pool)

	repo := NewRepository(pool)

	name := "New Name"
	_, err := repo.Update(ctx, userID, PatchFields{DisplayName: &name})
	if !errors.Is(err, ErrProfileNotFound) {
		t.Errorf("expected ErrProfileNotFound for a user with no listener_profiles row, got %v", err)
	}
}
