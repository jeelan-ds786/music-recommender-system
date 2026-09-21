package health

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type Check struct {
	Name string
	Ping func(context.Context) error
}

type Handler struct {
	timeout time.Duration
	checks  []Check
}

func New(timeout time.Duration, checks ...Check) *Handler {
	return &Handler{timeout: timeout, checks: checks}
}

func (h *Handler) Live(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) Ready(w http.ResponseWriter, request *http.Request) {
	failed := make([]string, 0, len(h.checks))
	for _, check := range h.checks {
		ctx, cancel := context.WithTimeout(request.Context(), h.timeout)
		err := check.Ping(ctx)
		cancel()
		if err != nil {
			failed = append(failed, check.Name)
		}
	}
	if len(failed) > 0 {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "unavailable", "failed": failed})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
