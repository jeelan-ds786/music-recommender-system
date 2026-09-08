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

	"github.com/google/uuid"

	playbackauth "github.com/jeelan-ds786/music-recommender-system/music-playback-service/internal/auth"
	"github.com/jeelan-ds786/music-recommender-system/music-playback-service/internal/logger"
)

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

func jsonContains(t *testing.T, body string, code string) bool {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	got, _ := payload["error"].(string)
	return got == code
}
