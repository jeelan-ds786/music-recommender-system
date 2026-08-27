package oauth

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

func TestPostgresAccountRepositoryCreatesAndFindsGoogleIdentity(t *testing.T) {
	_ = godotenv.Load("../../.env")
	dsn := os.Getenv("DB_URL")
	if dsn == "" {
		t.Skip("DB_URL not set, skipping integration test")
	}

	ctx := context.Background()
	pool, err := db.NewPostgresPool(ctx, dsn, nil)
	if err != nil {
		t.Fatalf("connect to postgres: %v", err)
	}
	t.Cleanup(pool.Close)

	repository := NewAccountRepository(pool)
	created := &user.User{
		ID:           uuid.New(),
		Email:        "oauth-account-" + uuid.NewString() + "@example.com",
		AuthProvider: googleProvider,
	}
	if err := repository.CreateUser(ctx, created, googleProvider, "google-subject"); err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", created.ID) })

	loaded, err := repository.GetUser(ctx, googleProvider, "google-subject")
	if err != nil {
		t.Fatalf("GetUser() error = %v", err)
	}
	if loaded.ID != created.ID || loaded.Email != created.Email || loaded.HashedPassword != "" {
		t.Fatalf("loaded user = %#v", loaded)
	}

	var passwordIsNull bool
	if err := pool.QueryRow(ctx, "SELECT hashed_password IS NULL FROM users WHERE id = $1", created.ID).Scan(&passwordIsNull); err != nil {
		t.Fatalf("query password nullability: %v", err)
	}
	if !passwordIsNull {
		t.Fatal("OAuth account has a password hash")
	}

	duplicate := &user.User{
		ID:           uuid.New(),
		Email:        "oauth-duplicate-" + uuid.NewString() + "@example.com",
		AuthProvider: googleProvider,
	}
	if err := repository.CreateUser(ctx, duplicate, googleProvider, "google-subject"); err == nil {
		t.Fatal("CreateUser() with duplicate provider subject succeeded")
	}
	var duplicateCount int
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM users WHERE id = $1", duplicate.ID).Scan(&duplicateCount); err != nil {
		t.Fatalf("query rolled back user: %v", err)
	}
	if duplicateCount != 0 {
		t.Fatal("user insert was not rolled back after provider account conflict")
	}

	if _, err := repository.GetUser(ctx, googleProvider, "missing-subject"); !errors.Is(err, user.ErrUserNotFound) {
		t.Fatalf("missing GetUser() error = %v, want ErrUserNotFound", err)
	}
}
