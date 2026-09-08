package httplog

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/jeelan-ds786/music-recommender-system/music-playback-service/internal/logger"
	"github.com/jeelan-ds786/music-recommender-system/music-playback-service/internal/reqid"
)

// Middleware emits one completion event per request. Headers and bodies are
// intentionally excluded so credentials and tokens cannot enter the logs.
// No metrics Observer parameter yet (unlike identity-gatekeeper's) — bounded-
// label Prometheus metrics are E3-SS-05 scope; this can grow the same
// variadic Observer hook then.
func Middleware(log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			started := time.Now()
			recorder := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(recorder, r)

			latency := time.Since(started)
			route := chi.RouteContext(r.Context()).RoutePattern()
			if route == "" {
				route = "unmatched"
			}

			rid, _ := reqid.FromContext(r.Context())
			fields := []logger.Field{
				logger.String("method", r.Method),
				logger.String("route", route),
				logger.Int("status", recorder.Status()),
				logger.Duration("latency", latency),
			}
			if userID, ok := reqid.UserIDFromContext(r.Context()); ok {
				fields = append(fields, logger.String("user_id", userID))
			}
			log.InfoFields(rid, "request completed", fields...)
		})
	}
}
