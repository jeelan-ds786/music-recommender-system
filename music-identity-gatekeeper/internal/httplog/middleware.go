package httplog

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/logger"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/reqid"
)

type Observer interface {
	ObserveHTTPRequest(method, route, status string, latency time.Duration)
}

// Middleware emits one completion event per request. Headers and bodies are
// intentionally excluded so credentials and tokens cannot enter the logs.
func Middleware(log *logger.Logger, observers ...Observer) func(http.Handler) http.Handler {
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
			status := strconv.Itoa(recorder.Status())
			for _, observer := range observers {
				observer.ObserveHTTPRequest(r.Method, route, status, latency)
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
