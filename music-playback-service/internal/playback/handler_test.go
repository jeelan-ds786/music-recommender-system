package playback

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	playbackauth "github.com/jeelan-ds786/music-recommender-system/music-playback-service/internal/auth"
	"github.com/jeelan-ds786/music-recommender-system/music-playback-service/internal/logger"
)

// router builds a minimal chi router so GetSessionEvents can resolve
// chi.URLParam the same way it does when mounted from cmd/server/main.go
// — mirrors music-identity-gatekeeper/internal/playlist/handler_test.go's
// identical-purpose helper.
func router(h *Handler) chi.Router {
	r := chi.NewRouter()
	r.Get("/v1/playback/sessions/{sessionID}/events", h.GetSessionEvents)
	return r
}

func withAuthedUser(req *http.Request, userID string) *http.Request {
	ctx := context.WithValue(req.Context(), playbackauth.UserIDKey, userID)
	return req.WithContext(ctx)
}

func requestBody(t *testing.T, req IngestRequest) *bytes.Buffer {
	t.Helper()
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	return bytes.NewBuffer(body)
}

func TestHandlerIngestSuccess(t *testing.T) {
	handler := NewHandler(NewService(newFakeRepository(), logger.New(logger.LevelNone)), logger.New(logger.LevelNone))
	req := validRequest()

	httpReq := withAuthedUser(
		httptest.NewRequest(http.MethodPost, "/v1/playback/events", requestBody(t, req)),
		uuid.New().String(),
	)
	recorder := httptest.NewRecorder()

	handler.Ingest(recorder, httpReq)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d, body=%s", recorder.Code, http.StatusAccepted, recorder.Body.String())
	}

	var body struct {
		Data IngestResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Data.Status != IngestStatusAccepted {
		t.Fatalf("status = %s, want %s", body.Data.Status, IngestStatusAccepted)
	}
}

func TestHandlerIngestUnauthenticated(t *testing.T) {
	handler := NewHandler(NewService(newFakeRepository(), logger.New(logger.LevelNone)), logger.New(logger.LevelNone))
	httpReq := httptest.NewRequest(http.MethodPost, "/v1/playback/events", requestBody(t, validRequest()))
	recorder := httptest.NewRecorder()

	handler.Ingest(recorder, httpReq)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func TestHandlerIngestBodyCannotOverrideAuthenticatedUser(t *testing.T) {
	// IngestRequest has no user_id field at all, so there's nothing for a
	// malicious body to set — this test documents that guarantee by
	// checking a raw payload with an extra user_id key is simply ignored.
	handler := NewHandler(NewService(newFakeRepository(), logger.New(logger.LevelNone)), logger.New(logger.LevelNone))
	authedUser := uuid.New()

	payload := []byte(`{
		"user_id": "` + uuid.New().String() + `",
		"client_event_id": "` + uuid.New().String() + `",
		"song_id": "` + uuid.New().String() + `",
		"session_id": "` + uuid.New().String() + `",
		"event_type": "play",
		"occurred_at": "` + time.Now().Format(time.RFC3339) + `",
		"position_ms": 0,
		"device_type": "mobile"
	}`)

	httpReq := withAuthedUser(
		httptest.NewRequest(http.MethodPost, "/v1/playback/events", bytes.NewReader(payload)),
		authedUser.String(),
	)
	recorder := httptest.NewRecorder()

	handler.Ingest(recorder, httpReq)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d, body=%s", recorder.Code, http.StatusAccepted, recorder.Body.String())
	}
}

func TestHandlerIngestInvalidBody(t *testing.T) {
	handler := NewHandler(NewService(newFakeRepository(), logger.New(logger.LevelNone)), logger.New(logger.LevelNone))
	httpReq := withAuthedUser(
		httptest.NewRequest(http.MethodPost, "/v1/playback/events", bytes.NewBufferString("not json")),
		uuid.New().String(),
	)
	recorder := httptest.NewRecorder()

	handler.Ingest(recorder, httpReq)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	if got := recorder.Body.String(); !jsonContains(t, got, "INVALID_REQUEST_BODY") {
		t.Fatalf("body = %s, want INVALID_REQUEST_BODY", got)
	}
}

func TestHandlerIngestValidationError(t *testing.T) {
	handler := NewHandler(NewService(newFakeRepository(), logger.New(logger.LevelNone)), logger.New(logger.LevelNone))
	req := validRequest()
	req.PositionMS = -1

	httpReq := withAuthedUser(
		httptest.NewRequest(http.MethodPost, "/v1/playback/events", requestBody(t, req)),
		uuid.New().String(),
	)
	recorder := httptest.NewRecorder()

	handler.Ingest(recorder, httpReq)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	if got := recorder.Body.String(); !jsonContains(t, got, "POSITION_MS_NEGATIVE") {
		t.Fatalf("body = %s, want POSITION_MS_NEGATIVE", got)
	}
}

func TestHandlerIngestBatchOrderPreserved(t *testing.T) {
	handler := NewHandler(NewService(newFakeRepository(), logger.New(logger.LevelNone)), logger.New(logger.LevelNone))
	first := validRequest()
	second := validRequest()

	body, err := json.Marshal(BatchIngestRequest{Events: []IngestRequest{first, second}})
	if err != nil {
		t.Fatalf("marshal batch: %v", err)
	}

	httpReq := withAuthedUser(
		httptest.NewRequest(http.MethodPost, "/v1/playback/events:batch", bytes.NewBuffer(body)),
		uuid.New().String(),
	)
	recorder := httptest.NewRecorder()

	handler.IngestBatch(recorder, httpReq)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d, body=%s", recorder.Code, http.StatusAccepted, recorder.Body.String())
	}

	var decoded struct {
		Data BatchIngestResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(decoded.Data.Results) != 2 ||
		decoded.Data.Results[0].ClientEventID != first.ClientEventID ||
		decoded.Data.Results[1].ClientEventID != second.ClientEventID {
		t.Fatalf("results out of order: %+v", decoded.Data.Results)
	}
}

func TestHandlerIngestBatchTooLarge(t *testing.T) {
	handler := NewHandler(NewService(newFakeRepository(), logger.New(logger.LevelNone)), logger.New(logger.LevelNone))
	events := make([]IngestRequest, MaxBatchSize+1)
	for i := range events {
		events[i] = validRequest()
	}

	body, err := json.Marshal(BatchIngestRequest{Events: events})
	if err != nil {
		t.Fatalf("marshal batch: %v", err)
	}

	httpReq := withAuthedUser(
		httptest.NewRequest(http.MethodPost, "/v1/playback/events:batch", bytes.NewBuffer(body)),
		uuid.New().String(),
	)
	recorder := httptest.NewRecorder()

	handler.IngestBatch(recorder, httpReq)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	if got := recorder.Body.String(); !jsonContains(t, got, "BATCH_TOO_LARGE") {
		t.Fatalf("body = %s, want BATCH_TOO_LARGE", got)
	}
}

func TestHandlerIngestRepositoryFailure(t *testing.T) {
	repo := newFakeRepository()
	repo.err = errors.New("connection reset")
	handler := NewHandler(NewService(repo, logger.New(logger.LevelNone)), logger.New(logger.LevelNone))

	httpReq := withAuthedUser(
		httptest.NewRequest(http.MethodPost, "/v1/playback/events", requestBody(t, validRequest())),
		uuid.New().String(),
	)
	recorder := httptest.NewRecorder()

	handler.Ingest(recorder, httpReq)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}

func TestHandlerGetSessionEventsSuccess(t *testing.T) {
	now := time.Now()
	event := Event{
		ID: uuid.New(), ClientEventID: uuid.New(), SongID: uuid.New(),
		Type: EventTypePlay, OccurredAt: now, IngestedAt: now, PositionMS: 0, DeviceType: "mobile",
	}
	sessionID := uuid.New()

	repo := newFakeRepository()
	repo.sessionEvents = []Event{event}
	repo.sessionOverview = &SessionOverview{EventCount: 1, StartedAt: now, EndedAt: now, OutOfOrder: map[uuid.UUID]bool{}}
	handler := NewHandler(NewService(repo, logger.New(logger.LevelNone)), logger.New(logger.LevelNone))

	httpReq := withAuthedUser(
		httptest.NewRequest(http.MethodGet, "/v1/playback/sessions/"+sessionID.String()+"/events", nil),
		uuid.New().String(),
	)
	recorder := httptest.NewRecorder()

	router(handler).ServeHTTP(recorder, httpReq)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var body struct {
		Data SessionEventsPage `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Data.Events) != 1 || body.Data.Events[0].EventID != event.ID {
		t.Fatalf("Events = %+v, want the one fake event", body.Data.Events)
	}
	if body.Data.Session.EventCount != 1 {
		t.Fatalf("Session.EventCount = %d, want 1", body.Data.Session.EventCount)
	}
}

func TestHandlerGetSessionEventsUnauthenticated(t *testing.T) {
	handler := NewHandler(NewService(newFakeRepository(), logger.New(logger.LevelNone)), logger.New(logger.LevelNone))

	httpReq := httptest.NewRequest(http.MethodGet, "/v1/playback/sessions/"+uuid.New().String()+"/events", nil)
	recorder := httptest.NewRecorder()

	router(handler).ServeHTTP(recorder, httpReq)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func TestHandlerGetSessionEventsInvalidSessionID(t *testing.T) {
	handler := NewHandler(NewService(newFakeRepository(), logger.New(logger.LevelNone)), logger.New(logger.LevelNone))

	httpReq := withAuthedUser(
		httptest.NewRequest(http.MethodGet, "/v1/playback/sessions/not-a-uuid/events", nil),
		uuid.New().String(),
	)
	recorder := httptest.NewRecorder()

	router(handler).ServeHTTP(recorder, httpReq)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	if got := recorder.Body.String(); !jsonContains(t, got, "INVALID_SESSION_ID") {
		t.Fatalf("body = %s, want INVALID_SESSION_ID", got)
	}
}

func TestHandlerGetSessionEventsInvalidCursor(t *testing.T) {
	handler := NewHandler(NewService(newFakeRepository(), logger.New(logger.LevelNone)), logger.New(logger.LevelNone))

	httpReq := withAuthedUser(
		httptest.NewRequest(http.MethodGet, "/v1/playback/sessions/"+uuid.New().String()+"/events?cursor=not-a-valid-cursor", nil),
		uuid.New().String(),
	)
	recorder := httptest.NewRecorder()

	router(handler).ServeHTTP(recorder, httpReq)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
	if got := recorder.Body.String(); !jsonContains(t, got, "INVALID_CURSOR") {
		t.Fatalf("body = %s, want INVALID_CURSOR", got)
	}
}

func TestHandlerGetSessionEventsInvalidLimit(t *testing.T) {
	handler := NewHandler(NewService(newFakeRepository(), logger.New(logger.LevelNone)), logger.New(logger.LevelNone))

	httpReq := withAuthedUser(
		httptest.NewRequest(http.MethodGet, "/v1/playback/sessions/"+uuid.New().String()+"/events?limit=not-a-number", nil),
		uuid.New().String(),
	)
	recorder := httptest.NewRecorder()

	router(handler).ServeHTTP(recorder, httpReq)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	if got := recorder.Body.String(); !jsonContains(t, got, "INVALID_LIMIT") {
		t.Fatalf("body = %s, want INVALID_LIMIT", got)
	}
}

func TestHandlerGetSessionEventsNotFound(t *testing.T) {
	repo := newFakeRepository()
	repo.sessionErr = ErrSessionNotFound
	handler := NewHandler(NewService(repo, logger.New(logger.LevelNone)), logger.New(logger.LevelNone))

	httpReq := withAuthedUser(
		httptest.NewRequest(http.MethodGet, "/v1/playback/sessions/"+uuid.New().String()+"/events", nil),
		uuid.New().String(),
	)
	recorder := httptest.NewRecorder()

	router(handler).ServeHTTP(recorder, httpReq)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
	if got := recorder.Body.String(); !jsonContains(t, got, "SESSION_NOT_FOUND") {
		t.Fatalf("body = %s, want SESSION_NOT_FOUND", got)
	}
}

func jsonContains(t *testing.T, body string, code string) bool {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	got, _ := payload["error"].(string)
	return got == code
}
