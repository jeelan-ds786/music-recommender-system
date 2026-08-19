package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/token"
)

type contextKey string

const UserIDKey contextKey = "user_id"

func UserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(UserIDKey).(string)
	return userID, ok
}

func AuthMiddleware(jwtService *token.JWTService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" {
				writeError(w, http.StatusUnauthorized, "MISSING_AUTHORIZATION_HEADER")
				return
			}

			const prefix = "Bearer "
			if !strings.HasPrefix(header, prefix) {
				writeError(w, http.StatusUnauthorized, "INVALID_AUTHORIZATION_HEADER")
				return
			}

			tokenString := strings.TrimSpace(strings.TrimPrefix(header, prefix))
			if tokenString == "" {
				writeError(w, http.StatusUnauthorized, "INVALID_AUTHORIZATION_HEADER")
				return
			}

			claims, err := jwtService.ParseAccessToken(tokenString)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "INVALID_ACCESS_TOKEN")
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
