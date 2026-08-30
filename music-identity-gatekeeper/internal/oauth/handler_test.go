package oauth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/token"
)

type fakeOAuthService struct {
	authorizationURL string
	pair             *token.TokenPair
	err              error
}

func (s fakeOAuthService) Begin(context.Context) (string, error) {
	return s.authorizationURL, s.err
}

func (s fakeOAuthService) Callback(context.Context, string, string) (*token.TokenPair, error) {
	return s.pair, s.err
}

func TestHandlerBeginRedirectsToProvider(t *testing.T) {
	handler := NewHandler(fakeOAuthService{authorizationURL: "https://accounts.google.test/authorize?state=opaque"})
	request := httptest.NewRequest(http.MethodGet, "/auth/google", nil)
	response := httptest.NewRecorder()

	handler.Begin(response, request)

	if response.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusFound)
	}
	if response.Header().Get("Location") != "https://accounts.google.test/authorize?state=opaque" {
		t.Fatalf("Location = %q", response.Header().Get("Location"))
	}
}

func TestHandlerCallbackReturnsTokensAsJSON(t *testing.T) {
	handler := NewHandler(fakeOAuthService{pair: &token.TokenPair{AccessToken: "access", RefreshToken: "refresh"}})
	request := httptest.NewRequest(http.MethodGet, "/auth/google/callback?code=code&state=state", nil)
	response := httptest.NewRecorder()

	handler.Callback(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if response.Header().Get("Location") != "" {
		t.Fatal("callback redirected with credentials")
	}
	if body := response.Body.String(); !strings.Contains(body, `"access_token":"access"`) || !strings.Contains(body, `"refresh_token":"refresh"`) {
		t.Fatalf("body = %q", body)
	}
}

func TestHandlerCallbackMapsOAuthErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{name: "invalid state", err: ErrInvalidState, wantStatus: http.StatusBadRequest, wantCode: "INVALID_OAUTH_CALLBACK"},
		{name: "unverified email", err: ErrEmailUnverified, wantStatus: http.StatusForbidden, wantCode: "OAUTH_EMAIL_UNVERIFIED"},
		{name: "local account conflict", err: ErrEmailConflict, wantStatus: http.StatusConflict, wantCode: "OAUTH_EMAIL_CONFLICT"},
		{name: "provider failure", err: errors.Join(ErrProviderFailure, errors.New("unavailable")), wantStatus: http.StatusBadGateway, wantCode: "OAUTH_PROVIDER_ERROR"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := NewHandler(fakeOAuthService{err: test.err})
			request := httptest.NewRequest(http.MethodGet, "/auth/google/callback?code=code&state=state", nil)
			response := httptest.NewRecorder()

			handler.Callback(response, request)

			if response.Code != test.wantStatus || !strings.Contains(response.Body.String(), test.wantCode) {
				t.Fatalf("response = %d %q", response.Code, response.Body.String())
			}
		})
	}
}
