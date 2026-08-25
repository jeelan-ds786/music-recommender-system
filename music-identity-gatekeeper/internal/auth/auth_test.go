package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
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

func (r *fakeUserRepository) CreateUser(_ context.Context, createdUser *user.User) error {
	r.createdUser = createdUser
	return nil
}

func (r *fakeUserRepository) GetByEmail(_ context.Context, email string) (*user.User, error) {
	r.lookupEmail = email
	return r.user, r.lookupErr
}

func (r *fakeUserRepository) GetByID(context.Context, uuid.UUID) (*user.User, error) {
	return nil, user.ErrUserNotFound
}

type fakeTierProvider struct{ tier string }

func (p fakeTierProvider) GetTier(context.Context, uuid.UUID) (string, error) {
	return p.tier, nil
}

type fakeRefreshRepository struct{}

func (fakeRefreshRepository) CreateRefreshToken(context.Context, *refresh.RefreshToken) error {
	return nil
}

func (fakeRefreshRepository) GetByHash(context.Context, string) (*refresh.RefreshToken, error) {
	return nil, errors.New("not found")
}

func (fakeRefreshRepository) Revoke(context.Context, string) error { return nil }

func (fakeRefreshRepository) Rotate(context.Context, string, *refresh.RefreshToken) error {
	return nil
}

func newTestService(repo user.Repository) Service {
	jwtService := token.NewJWTService("test-secret")
	return NewService(
		repo,
		token.NewService(jwtService, fakeRefreshRepository{}, fakeTierProvider{tier: "free"}),
		logger.New(logger.LevelNone),
	)
}

func TestRegister(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		existing   *user.User
		lookupErr  error
		wantStatus int
		wantError  string
		wantField  string
	}{
		{
			name:       "successful registration",
			body:       `{"email":"LISTENER@Example.COM","password":"strong-password"}`,
			lookupErr:  user.ErrUserNotFound,
			wantStatus: http.StatusCreated,
		},
		{
			name:       "duplicate email",
			body:       `{"email":"listener@example.com","password":"strong-password"}`,
			existing:   &user.User{ID: uuid.New(), Email: "listener@example.com"},
			wantStatus: http.StatusConflict,
			wantError:  "EMAIL_ALREADY_EXISTS",
		},
		{
			name:       "invalid email",
			body:       `{"email":"not-an-email","password":"strong-password"}`,
			wantStatus: http.StatusBadRequest,
			wantError:  "EMAIL_INVALID",
			wantField:  "email",
		},
		{
			name:       "weak password",
			body:       `{"email":"listener@example.com","password":"short"}`,
			wantStatus: http.StatusBadRequest,
			wantError:  "PASSWORD_TOO_SHORT",
			wantField:  "password",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := &fakeUserRepository{user: test.existing, lookupErr: test.lookupErr}
			handler := NewHandler(newTestService(repo))
			request := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBufferString(test.body))
			response := httptest.NewRecorder()

			handler.Register(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}

			var payload map[string]any
			if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if payload["error"] != test.wantError && test.wantError != "" {
				t.Errorf("error = %v, want %q", payload["error"], test.wantError)
			}
			if payload["field"] != test.wantField && test.wantField != "" {
				t.Errorf("field = %v, want %q", payload["field"], test.wantField)
			}

			if test.name == "successful registration" {
				if repo.lookupEmail != "listener@example.com" {
					t.Errorf("lookup email = %q", repo.lookupEmail)
				}
				if repo.createdUser == nil || repo.createdUser.Email != "listener@example.com" {
					t.Errorf("created user email was not normalized")
				}
			}
		})
	}
}

func TestLogin(t *testing.T) {
	passwordHash, err := HashPassword("correct-password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	tests := []struct {
		name       string
		email      string
		password   string
		storedUser *user.User
		lookupErr  error
		wantStatus int
		wantError  string
	}{
		{
			name:       "successful login",
			email:      "LISTENER@Example.COM",
			password:   "correct-password",
			storedUser: &user.User{ID: uuid.New(), Email: "listener@example.com", HashedPassword: passwordHash, AuthProvider: "local"},
			wantStatus: http.StatusOK,
		},
		{
			name:       "wrong password",
			email:      "listener@example.com",
			password:   "wrong-password",
			storedUser: &user.User{ID: uuid.New(), Email: "listener@example.com", HashedPassword: passwordHash, AuthProvider: "local"},
			wantStatus: http.StatusUnauthorized,
			wantError:  "INVALID_CREDENTIALS",
		},
		{
			name:       "unknown email",
			email:      "unknown@example.com",
			password:   "wrong-password",
			lookupErr:  user.ErrUserNotFound,
			wantStatus: http.StatusUnauthorized,
			wantError:  "INVALID_CREDENTIALS",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := &fakeUserRepository{user: test.storedUser, lookupErr: test.lookupErr}
			handler := NewHandler(newTestService(repo))
			body, _ := json.Marshal(LoginRequest{Email: test.email, Password: test.password})
			request := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
			response := httptest.NewRecorder()

			handler.Login(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}

			var payload map[string]any
			if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if payload["error"] != test.wantError && test.wantError != "" {
				t.Errorf("error = %v, want %q", payload["error"], test.wantError)
			}
			if test.name == "successful login" {
				if repo.lookupEmail != "listener@example.com" {
					t.Errorf("lookup email = %q", repo.lookupEmail)
				}
				if payload["access_token"] == "" || payload["refresh_token"] == "" {
					t.Errorf("successful login did not return a token pair")
				}
			}
		})
	}
}

func TestUnknownUserLoginUsesDummyBcryptHash(t *testing.T) {
	repo := &fakeUserRepository{lookupErr: user.ErrUserNotFound}
	service := newTestService(repo).(*AuthService)
	comparisonCalls := 0
	service.comparePassword = func(hash, password string) error {
		comparisonCalls++
		if hash != dummyPasswordHash {
			t.Errorf("hash = %q, want dummy hash", hash)
		}
		if password != "submitted-password" {
			t.Errorf("password = %q", password)
		}
		return errors.New("password mismatch")
	}

	_, err := service.Login(context.Background(), LoginRequest{
		Email:    "  MISSING@Example.COM ",
		Password: "submitted-password",
	})

	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("error = %v, want ErrInvalidCredentials", err)
	}
	if comparisonCalls != 1 {
		t.Fatalf("comparison calls = %d, want 1", comparisonCalls)
	}
	if repo.lookupEmail != "missing@example.com" {
		t.Fatalf("lookup email = %q, want normalized email", repo.lookupEmail)
	}
}

func TestDummyPasswordHashIsValid(t *testing.T) {
	cost, err := bcrypt.Cost([]byte(dummyPasswordHash))
	if err != nil {
		t.Fatalf("dummy hash is not valid bcrypt: %v", err)
	}
	if cost != 12 {
		t.Fatalf("dummy hash cost = %d, want 12", cost)
	}
}
