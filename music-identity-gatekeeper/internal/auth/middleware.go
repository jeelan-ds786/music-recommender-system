package auth

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/logger"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/reqid"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/token"
)

type contextKey string

const (
	UserIDKey contextKey = "user_id"
	TierKey   contextKey = "tier"
	JTIKey    contextKey = "jti"
	ExpiryKey contextKey = "expires_at"
)

func UserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(UserIDKey).(string)
	return userID, ok
}

func TierFromContext(ctx context.Context) (string, bool) {
	tier, ok := ctx.Value(TierKey).(string)
	return tier, ok
}

func JTIFromContext(ctx context.Context) (string, bool) {
	jti, ok := ctx.Value(JTIKey).(string)
	return jti, ok
}

func ExpiryFromContext(ctx context.Context) (time.Time, bool) {
	expiresAt, ok := ctx.Value(ExpiryKey).(time.Time)
	return expiresAt, ok
}

type BlacklistChecker interface {
	IsBlacklisted(ctx context.Context, jti string) (bool, error)
}

func AuthMiddleware(
	jwtService *token.JWTService,
	log *logger.Logger,
	blacklists ...BlacklistChecker,
) func(http.Handler) http.Handler {
	var blacklist BlacklistChecker
	if len(blacklists) > 0 {
		blacklist = blacklists[0]
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rid, _ := reqid.FromContext(r.Context())

			log.Debug(rid, "Starting session validation for path=%s", r.URL.Path)

			header := r.Header.Get("Authorization")
			if header == "" {
				log.Error(rid, "Ending session validation for path=%s (invalid session: missing authorization header)", r.URL.Path)
				writeError(w, http.StatusUnauthorized, "MISSING_AUTHORIZATION_HEADER")
				return
			}

			const prefix = "Bearer "
			if !strings.HasPrefix(header, prefix) {
				log.Error(rid, "Ending session validation for path=%s (invalid session: malformed authorization header)", r.URL.Path)
				writeError(w, http.StatusUnauthorized, "INVALID_AUTHORIZATION_HEADER")
				return
			}

			tokenString := strings.TrimSpace(strings.TrimPrefix(header, prefix))
			if tokenString == "" {
				log.Error(rid, "Ending session validation for path=%s (invalid session: empty bearer token)", r.URL.Path)
				writeError(w, http.StatusUnauthorized, "INVALID_AUTHORIZATION_HEADER")
				return
			}

			log.Debug(rid, "parsing access token for path=%s", r.URL.Path)

			claims, err := jwtService.ParseAccessToken(tokenString)
			if err != nil {
				log.Error(rid, "Ending session validation for path=%s (invalid session: %v)", r.URL.Path, err)
				writeError(w, http.StatusUnauthorized, "INVALID_ACCESS_TOKEN")
				return
			}
			if blacklist != nil {
				blacklisted, err := blacklist.IsBlacklisted(r.Context(), claims.ID)
				if err != nil {
					log.Error(rid, "Ending session validation for path=%s (revocation check unavailable)", r.URL.Path)
					writeError(w, http.StatusServiceUnavailable, "AUTHORIZATION_UNAVAILABLE")
					return
				}
				if blacklisted {
					log.Info(rid, "Ending session validation for path=%s (revoked access token)", r.URL.Path)
					writeError(w, http.StatusUnauthorized, "REVOKED_ACCESS_TOKEN")
					return
				}
			}

			log.Info(rid, "Ending session validation for path=%s (valid session for user_id=%s)", r.URL.Path, claims.UserID)

			ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
			ctx = context.WithValue(ctx, TierKey, claims.Tier)
			ctx = context.WithValue(ctx, JTIKey, claims.ID)
			ctx = context.WithValue(ctx, ExpiryKey, claims.ExpiresAt.Time)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func TierMiddleware(allowedTiers ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(allowedTiers))
	for _, tier := range allowedTiers {
		allowed[tier] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tier, ok := TierFromContext(r.Context())
			if !ok {
				writeError(w, http.StatusForbidden, "INSUFFICIENT_TIER")
				return
			}
			if _, ok := allowed[tier]; !ok {
				writeError(w, http.StatusForbidden, "INSUFFICIENT_TIER")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
