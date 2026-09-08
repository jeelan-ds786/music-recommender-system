package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"

	"github.com/jeelan-ds786/music-recommender-system/music-playback-service/internal/logger"
	"github.com/jeelan-ds786/music-recommender-system/music-playback-service/internal/reqid"
	"github.com/jeelan-ds786/music-recommender-system/music-playback-service/internal/response"
)

type contextKey string

const UserIDKey contextKey = "user_id"

func UserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(UserIDKey).(string)
	return userID, ok
}

// Middleware verifies the bearer access token issued by
// music-identity-gatekeeper and puts the authenticated user ID in the
// request context. It never trusts a request-body/path field for
// identity — user_id always comes from here. Logging mirrors
// music-identity-gatekeeper's AuthMiddleware exactly (same Starting/Ending
// session-validation convention).
func Middleware(secret string, log *logger.Logger) func(http.Handler) http.Handler {
	key := []byte(secret)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rid, _ := reqid.FromContext(r.Context())

			log.Debug(rid, "Starting session validation for path=%s", r.URL.Path)

			header := r.Header.Get("Authorization")
			if header == "" {
				log.Error(rid, "Ending session validation for path=%s (invalid session: missing authorization header)", r.URL.Path)
				response.Error(w, http.StatusUnauthorized, "MISSING_AUTHORIZATION_HEADER")
				return
			}

			const prefix = "Bearer "
			if !strings.HasPrefix(header, prefix) {
				log.Error(rid, "Ending session validation for path=%s (invalid session: malformed authorization header)", r.URL.Path)
				response.Error(w, http.StatusUnauthorized, "INVALID_AUTHORIZATION_HEADER")
				return
			}

			tokenString := strings.TrimSpace(strings.TrimPrefix(header, prefix))
			if tokenString == "" {
				log.Error(rid, "Ending session validation for path=%s (invalid session: empty bearer token)", r.URL.Path)
				response.Error(w, http.StatusUnauthorized, "INVALID_AUTHORIZATION_HEADER")
				return
			}

			claims := &Claims{}
			token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
				if token.Method != jwt.SigningMethodHS256 {
					return nil, jwt.ErrTokenSignatureInvalid
				}
				return key, nil
			})
			if err != nil || !token.Valid || claims.UserID == "" {
				log.Error(rid, "Ending session validation for path=%s (invalid session: %v)", r.URL.Path, err)
				response.Error(w, http.StatusUnauthorized, "INVALID_ACCESS_TOKEN")
				return
			}

			log.Info(rid, "Ending session validation for path=%s (valid session for user_id=%s)", r.URL.Path, claims.UserID)
			reqid.SetUserID(r.Context(), claims.UserID)

			ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
