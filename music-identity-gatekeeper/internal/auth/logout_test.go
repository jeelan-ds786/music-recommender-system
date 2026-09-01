package auth

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/token"
)

type logoutService struct {
	request   LogoutRequest
	userID    uuid.UUID
	jti       string
	expiresAt time.Time
	err       error
}

func (s *logoutService) Register(context.Context, RegisterRequest) (*RegisterResponse, error) {
	return nil, nil
}

func (s *logoutService) Login(context.Context, LoginRequest) (*token.TokenPair, error) {
	return nil, nil
}

func (s *logoutService) Refresh(context.Context, RefreshRequest) (*token.TokenPair, error) {
	return nil, nil
}

func (s *logoutService) Logout(
	_ context.Context,
	request LogoutRequest,
	userID uuid.UUID,
	jti string,
	expiresAt time.Time,
) error {
	s.request = request
	s.userID = userID
	s.jti = jti
	s.expiresAt = expiresAt
	return s.err
}

func TestLogoutHandlerUsesAuthenticatedClaims(t *testing.T) {
	userID := uuid.New()
	expiresAt := time.Now().Add(10 * time.Minute)
	service := &logoutService{}
	handler := NewHandler(service)
	request := httptest.NewRequest(
		http.MethodPost,
		"/auth/logout",
		bytes.NewBufferString(`{"refresh_token":"raw-refresh-token"}`),
	)
	ctx := context.WithValue(request.Context(), UserIDKey, userID.String())
	ctx = context.WithValue(ctx, JTIKey, "access-token-id")
	ctx = context.WithValue(ctx, ExpiryKey, expiresAt)
	response := httptest.NewRecorder()

	handler.Logout(response, request.WithContext(ctx))

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
	if response.Body.Len() != 0 {
		t.Fatalf("response body = %q, want empty", response.Body.String())
	}
	if service.request.RefreshToken != "raw-refresh-token" {
		t.Fatalf("refresh token was not forwarded to the service")
	}
	if service.userID != userID || service.jti != "access-token-id" || !service.expiresAt.Equal(expiresAt) {
		t.Fatalf("logout claims = user %s, jti %q, expiry %v", service.userID, service.jti, service.expiresAt)
	}
}

func TestLogoutHandlerRejectsMissingAuthenticatedClaims(t *testing.T) {
	handler := NewHandler(&logoutService{})
	request := httptest.NewRequest(
		http.MethodPost,
		"/auth/logout",
		bytes.NewBufferString(`{"refresh_token":"raw-refresh-token"}`),
	)
	response := httptest.NewRecorder()

	handler.Logout(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}
