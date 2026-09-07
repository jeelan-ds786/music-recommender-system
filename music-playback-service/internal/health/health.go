package health

import (
	"context"
	"net/http"
	"time"

	"github.com/jeelan-ds786/music-recommender-system/music-playback-service/internal/response"
)

type Check struct {
	Name string
	Ping func(context.Context) error
}

type Handler struct {
	timeout time.Duration
	checks  []Check
}

func NewHandler(timeout time.Duration, checks ...Check) *Handler {
	return &Handler{timeout: timeout, checks: checks}
}

func (handler *Handler) Live(w http.ResponseWriter, _ *http.Request) {
	response.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (handler *Handler) Ready(w http.ResponseWriter, request *http.Request) {
	failed := make([]string, 0, len(handler.checks))
	for _, check := range handler.checks {
		ctx, cancel := context.WithTimeout(request.Context(), handler.timeout)
		err := check.Ping(ctx)
		cancel()
		if err != nil {
			failed = append(failed, check.Name)
		}
	}

	if len(failed) > 0 {
		response.JSON(w, http.StatusServiceUnavailable, map[string]any{
			"status": "unavailable",
			"failed": failed,
		})
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
