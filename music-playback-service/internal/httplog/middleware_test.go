package httplog

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/jeelan-ds786/music-recommender-system/music-playback-service/internal/logger"
	"github.com/jeelan-ds786/music-recommender-system/music-playback-service/internal/reqid"
)

func TestMiddlewareWritesOneSecureCompletionLog(t *testing.T) {
	var output bytes.Buffer
	log := logger.NewWithWriter(logger.LevelInfo, &output)
	router := chi.NewRouter()
	router.Use(reqid.Middleware)
	router.Use(Middleware(log))
	router.Post("/v1/playback/events", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})

	request := httptest.NewRequest(http.MethodPost, "/v1/playback/events", strings.NewReader(`{"context":{"secret-value":true}}`))
	request.Header.Set("Authorization", "Bearer access-token-value")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	lines := bytes.Split(bytes.TrimSpace(output.Bytes()), []byte("\n"))
	if len(lines) != 1 {
		t.Fatalf("log lines = %d, want 1: %s", len(lines), output.String())
	}
	if strings.Contains(output.String(), "secret-value") || strings.Contains(output.String(), "access-token-value") {
		t.Fatalf("log contains sensitive request data: %s", output.String())
	}

	var entry map[string]any
	if err := json.Unmarshal(lines[0], &entry); err != nil {
		t.Fatalf("log is not JSON: %v", err)
	}
	if entry["request_id"] == "" {
		t.Error("request_id is missing")
	}
	if entry["route"] != "/v1/playback/events" || entry["status"] != float64(http.StatusAccepted) {
		t.Fatalf("unexpected completion fields: %v", entry)
	}
	if response.Header().Get(reqid.HeaderName) == "" {
		t.Error("response request ID header is missing")
	}
}
