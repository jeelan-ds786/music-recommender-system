package authz

import (
	"crypto/subtle"
	"net/http"

	"github.com/jeelan-ds786/music-recommender-system/music-catalog-service/internal/response"
)

const AdminKeyHeader = "X-Admin-Key"

func AdminKeyMiddleware(adminKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			providedKey := r.Header.Get(AdminKeyHeader)
			if adminKey == "" || subtle.ConstantTimeCompare([]byte(providedKey), []byte(adminKey)) != 1 {
				response.Error(w, http.StatusUnauthorized, "unauthorized")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
