package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/jeelan-ds786/music-recommender-system/music-playback-service/internal/logger"
)

func sign(t *testing.T, secret string, claims Claims) string {
	t.Helper()
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return token
}

func TestMiddlewareAcceptsValidToken(t *testing.T) {
	const secret = "test-secret"
	token := sign(t, secret, Claims{
		UserID: "user-123",
		Tier:   "free",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	})

	handler := Middleware(secret, logger.New(logger.LevelNone))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := UserIDFromContext(r.Context())
		if !ok || userID != "user-123" {
			t.Fatalf("UserIDFromContext() = %q, %t", userID, ok)
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	request := httptest.NewRequest(http.MethodPost, "/v1/playback/events", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
}

func TestMiddlewareRejects(t *testing.T) {
	const secret = "test-secret"
	expired := sign(t, secret, Claims{
		UserID: "user-123",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
		},
	})
	wrongSecret := sign(t, "other-secret", Claims{
		UserID: "user-123",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	})

	tests := []struct {
		name       string
		authHeader string
		wantError  string
	}{
		{name: "missing header", authHeader: "", wantError: "MISSING_AUTHORIZATION_HEADER"},
		{name: "malformed header", authHeader: "Token abc", wantError: "INVALID_AUTHORIZATION_HEADER"},
		{name: "empty bearer", authHeader: "Bearer ", wantError: "INVALID_AUTHORIZATION_HEADER"},
		{name: "expired token", authHeader: "Bearer " + expired, wantError: "INVALID_ACCESS_TOKEN"},
		{name: "wrong signature", authHeader: "Bearer " + wrongSecret, wantError: "INVALID_ACCESS_TOKEN"},
		{name: "garbage token", authHeader: "Bearer not-a-jwt", wantError: "INVALID_ACCESS_TOKEN"},
	}

	handler := Middleware(secret, logger.New(logger.LevelNone))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be reached")
	}))

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/v1/playback/events", nil)
			if test.authHeader != "" {
				request.Header.Set("Authorization", test.authHeader)
			}
			recorder := httptest.NewRecorder()

			handler.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
			}
			if got := recorder.Body.String(); !strings.Contains(got, test.wantError) {
				t.Fatalf("body = %s, want to contain %q", got, test.wantError)
			}
		})
	}
}
