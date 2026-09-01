package reqid

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

type contextKey string

const RequestIDKey contextKey = "request_id"

const metadataKey contextKey = "request_metadata"

type metadata struct {
	userID string
}

const HeaderName = "X-Request-ID"

func FromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(RequestIDKey).(string)
	return id, ok
}

func SetUserID(ctx context.Context, userID string) {
	if value, ok := ctx.Value(metadataKey).(*metadata); ok {
		value.userID = userID
	}
}

func UserIDFromContext(ctx context.Context) (string, bool) {
	value, ok := ctx.Value(metadataKey).(*metadata)
	return value.userID, ok && value.userID != ""
}

// Middleware assigns each request a request ID: it reuses the caller's
// X-Request-ID header when present, otherwise generates one. The ID is
// echoed back in the response header and stored in the request context
// so handlers/services can log with it.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(HeaderName)
		if id == "" {
			id = uuid.NewString()
		}

		w.Header().Set(HeaderName, id)

		ctx := context.WithValue(r.Context(), RequestIDKey, id)
		ctx = context.WithValue(ctx, metadataKey, &metadata{})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
