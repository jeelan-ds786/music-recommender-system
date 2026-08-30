package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/token"
)

type OAuthService interface {
	Begin(ctx context.Context) (string, error)
	Callback(ctx context.Context, code, state string) (*token.TokenPair, error)
}

type Handler struct {
	service OAuthService
}

func NewHandler(service OAuthService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Begin(w http.ResponseWriter, r *http.Request) {
	authorizationURL, err := h.service.Begin(r.Context())
	if err != nil {
		writeOAuthError(w, http.StatusServiceUnavailable, "OAUTH_UNAVAILABLE")
		return
	}
	http.Redirect(w, r, authorizationURL, http.StatusFound)
}

func (h *Handler) Callback(w http.ResponseWriter, r *http.Request) {
	pair, err := h.service.Callback(r.Context(), r.URL.Query().Get("code"), r.URL.Query().Get("state"))
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidState), errors.Is(err, ErrMissingCode):
			writeOAuthError(w, http.StatusBadRequest, "INVALID_OAUTH_CALLBACK")
		case errors.Is(err, ErrEmailUnverified):
			writeOAuthError(w, http.StatusForbidden, "OAUTH_EMAIL_UNVERIFIED")
		case errors.Is(err, ErrEmailConflict):
			writeOAuthError(w, http.StatusConflict, "OAUTH_EMAIL_CONFLICT")
		case errors.Is(err, ErrProviderFailure):
			writeOAuthError(w, http.StatusBadGateway, "OAUTH_PROVIDER_ERROR")
		default:
			writeOAuthError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR")
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"access_token":  pair.AccessToken,
		"refresh_token": pair.RefreshToken,
	})
}

func writeOAuthError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code})
}
