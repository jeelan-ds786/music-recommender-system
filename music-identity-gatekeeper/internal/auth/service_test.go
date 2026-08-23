package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/logger"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/refresh"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/token"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/user"
)

type fakeUserRepository struct {
	user        *user.User
	lookupErr   error
	lookupEmail string
	createdUser *user.User
}

func (r *fakeUserRepository) CreateUser(_ context.Context, newUser *user.User) error {
	r.createdUser = newUser
	return nil
}

func (r *fakeUserRepository) GetByEmail(_ context.Context, email string) (*user.User, error) {
	r.lookupEmail = email
	return r.user, r.lookupErr
}

func (r *fakeUserRepository) GetByID(context.Context, uuid.UUID) (*user.User, error) {
	return nil, user.ErrUserNotFound
}

type createOnlyRefreshRepository struct{}

func (createOnlyRefreshRepository) CreateRefreshToken(context.Context, *refresh.RefreshToken) error {
	return nil
}

func (createOnlyRefreshRepository) GetByHash(context.Context, string) (*refresh.RefreshToken, error) {
	return nil, errors.New("not implemented")
}

func (createOnlyRefreshRepository) Revoke(context.Context, string) error {
	return errors.New("not implemented")
}

func (createOnlyRefreshRepository) Rotate(context.Context, string, *refresh.RefreshToken) error {
	return errors.New("not implemented")
}

func TestRegisterNormalizesEmailBeforeLookupAndInsert(t *testing.T) {
	repository := &fakeUserRepository{lookupErr: user.ErrUserNotFound}
	service := NewService(repository, nil, logger.New(logger.LevelNone))

	response, err := service.Register(context.Background(), RegisterRequest{
		Email: "  Listener@Example.COM ", Password: "strong-password",
	})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if repository.lookupEmail != "listener@example.com" {
		t.Fatalf("lookup email = %q", repository.lookupEmail)
	}
	if repository.createdUser == nil || repository.createdUser.Email != "listener@example.com" {
		t.Fatalf("created user email = %v", repository.createdUser)
	}
	if response.Email != "listener@example.com" {
		t.Fatalf("response email = %q", response.Email)
	}
}

func TestLoginCredentialChecks(t *testing.T) {
	knownUser := &user.User{ID: uuid.New(), Email: "listener@example.com", AuthProvider: "local"}
	tests := []struct {
		name             string
		repository       *fakeUserRepository
		compareErr       error
		wantHash         string
		wantErr          error
		wantToken        bool
		wantComparedOnce bool
	}{
		{
			name: "successful login", repository: &fakeUserRepository{user: knownUser},
			wantToken: true, wantComparedOnce: true,
		},
		{
			name: "wrong password", repository: &fakeUserRepository{user: knownUser},
			compareErr: errors.New("password mismatch"), wantErr: ErrInvalidCredentials, wantComparedOnce: true,
		},
		{
			name: "unknown email", repository: &fakeUserRepository{lookupErr: user.ErrUserNotFound},
			compareErr: errors.New("password mismatch"), wantHash: dummyPasswordHash,
			wantErr: ErrInvalidCredentials, wantComparedOnce: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tokenService := token.NewService(token.NewJWTService("test-secret"), createOnlyRefreshRepository{})
			service := NewService(test.repository, tokenService, logger.New(logger.LevelNone)).(*AuthService)
			compareCalls := 0
			service.comparePassword = func(hash, _ string) error {
				compareCalls++
				if test.wantHash != "" && hash != test.wantHash {
					t.Errorf("comparison hash = %q, want dummy hash", hash)
				}
				return test.compareErr
			}

			pair, err := service.Login(context.Background(), LoginRequest{
				Email: "  Listener@Example.COM ", Password: "password",
			})

			if !errors.Is(err, test.wantErr) {
				t.Fatalf("Login() error = %v, want %v", err, test.wantErr)
			}
			if (pair != nil) != test.wantToken {
				t.Fatalf("Login() token present = %t, want %t", pair != nil, test.wantToken)
			}
			if test.repository.lookupEmail != "listener@example.com" {
				t.Fatalf("lookup email = %q", test.repository.lookupEmail)
			}
			if test.wantComparedOnce && compareCalls != 1 {
				t.Fatalf("password comparisons = %d, want 1", compareCalls)
			}
		})
	}
}

func TestDummyPasswordHashIsValid(t *testing.T) {
	err := ComparePassword(dummyPasswordHash, "not-the-dummy-password")
	if !errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		t.Fatalf("dummy password comparison error = %v, want password mismatch", err)
	}
}
