package user

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/joho/godotenv"

	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/db"
)

func TestPostgresRepositoryStoresOAuthUserWithoutPasswordHash(t *testing.T) {
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

	repository := NewRepository(pool)
	created := &User{
		ID:           uuid.New(),
		Email:        "oauth-test-" + uuid.NewString() + "@example.com",
		AuthProvider: "google",
	}
	if err := repository.CreateUser(ctx, created); err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", created.ID)
	})

	var passwordIsNull bool
	if err := pool.QueryRow(ctx, "SELECT hashed_password IS NULL FROM users WHERE id = $1", created.ID).Scan(&passwordIsNull); err != nil {
		t.Fatalf("query password nullability: %v", err)
	}
	if !passwordIsNull {
		t.Fatal("OAuth user password hash is not SQL NULL")
	}

	loaded, err := repository.GetByEmail(ctx, created.Email)
	if err != nil {
		t.Fatalf("GetByEmail() error = %v", err)
	}
	if loaded.ID != created.ID || loaded.HashedPassword != "" || loaded.AuthProvider != "google" {
		t.Fatalf("loaded user = %#v", loaded)
	}
}
