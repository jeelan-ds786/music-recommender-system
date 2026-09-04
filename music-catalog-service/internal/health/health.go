package health

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/jeelan-ds786/music-recommender-system/music-catalog-service/internal/response"
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

func (h *Handler) Live(w http.ResponseWriter, _ *http.Request) {
	response.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	failed := make(chan string, len(h.checks))
	var waitGroup sync.WaitGroup

	for _, check := range h.checks {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
			defer cancel()
			if err := check.Ping(ctx); err != nil {
				failed <- check.Name
			}
		}()
	}

	waitGroup.Wait()
	close(failed)

	failures := make([]string, 0, len(failed))
	for name := range failed {
		failures = append(failures, name)
	}
	if len(failures) > 0 {
		response.JSON(w, http.StatusServiceUnavailable, map[string]any{
			"status": "unavailable",
			"failed": failures,
		})
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
