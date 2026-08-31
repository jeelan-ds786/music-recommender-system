package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/logger"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/token"
)

type fakeBlacklistChecker struct {
	blacklisted bool
	err         error
}

func (c fakeBlacklistChecker) IsBlacklisted(context.Context, string) (bool, error) {
	return c.blacklisted, c.err
}

func TestTierMiddleware(t *testing.T) {
	jwtService := token.NewJWTService("test-secret")
	premiumOnly := AuthMiddleware(jwtService, logger.New(logger.LevelNone))(TierMiddleware("premium")(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, userOK := UserIDFromContext(r.Context())
			tier, tierOK := TierFromContext(r.Context())
			if !userOK || !tierOK || userID != "user-123" || tier != "premium" {
				t.Fatalf("authenticated context = user %q/%t, tier %q/%t", userID, userOK, tier, tierOK)
			}
			w.WriteHeader(http.StatusNoContent)
		}),
	))

	tests := []struct {
		name       string
		signedTier string
		spoofTier  bool
		wantStatus int
		wantError  string
	}{
		{name: "free user denied", signedTier: "free", wantStatus: http.StatusForbidden, wantError: "INSUFFICIENT_TIER"},
		{name: "premium user allowed", signedTier: "premium", wantStatus: http.StatusNoContent},
		{name: "headers and body cannot override token", signedTier: "free", spoofTier: true, wantStatus: http.StatusForbidden, wantError: "INSUFFICIENT_TIER"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			accessToken, err := jwtService.GenerateAccessToken("user-123", test.signedTier)
			if err != nil {
				t.Fatalf("GenerateAccessToken() error = %v", err)
			}
			request := httptest.NewRequest(http.MethodPost, "/premium", bytes.NewBufferString(`{"tier":"premium"}`))
			request.Header.Set("Authorization", "Bearer "+accessToken)
			if test.spoofTier {
				request.Header.Set("X-Subscription-Tier", "premium")
			}
			response := httptest.NewRecorder()

			premiumOnly.ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
			if test.wantError != "" {
				var payload map[string]any
				if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
					t.Fatalf("decode error response: %v", err)
				}
				if payload["error"] != test.wantError {
					t.Fatalf("error = %v, want %q", payload["error"], test.wantError)
				}
			}
		})
	}
}

func TestAuthMiddlewareChecksAccessTokenBlacklist(t *testing.T) {
	jwtService := token.NewJWTService("test-secret")
	accessToken, err := jwtService.GenerateAccessToken("user-123", "free")
	if err != nil {
		t.Fatalf("GenerateAccessToken() error = %v", err)
	}

	tests := []struct {
		name       string
		checker    fakeBlacklistChecker
		wantStatus int
		wantError  string
	}{
		{name: "active token allowed", wantStatus: http.StatusNoContent},
		{name: "blacklisted token rejected", checker: fakeBlacklistChecker{blacklisted: true}, wantStatus: http.StatusUnauthorized, wantError: "REVOKED_ACCESS_TOKEN"},
		{name: "redis failure fails closed", checker: fakeBlacklistChecker{err: errors.New("redis unavailable")}, wantStatus: http.StatusServiceUnavailable, wantError: "AUTHORIZATION_UNAVAILABLE"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := AuthMiddleware(jwtService, logger.New(logger.LevelNone), test.checker)(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					jti, jtiOK := JTIFromContext(r.Context())
					expiresAt, expiryOK := ExpiryFromContext(r.Context())
					if !jtiOK || jti == "" || !expiryOK || !expiresAt.After(time.Now()) {
						t.Fatalf("revocation context = jti %q/%t, expiry %v/%t", jti, jtiOK, expiresAt, expiryOK)
					}
					w.WriteHeader(http.StatusNoContent)
				}),
			)
			request := httptest.NewRequest(http.MethodGet, "/me", nil)
			request.Header.Set("Authorization", "Bearer "+accessToken)
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
			if test.wantError != "" {
				var payload map[string]string
				if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
					t.Fatalf("decode error response: %v", err)
				}
				if payload["error"] != test.wantError {
					t.Fatalf("error = %q, want %q", payload["error"], test.wantError)
				}
			}
		})
	}
}
