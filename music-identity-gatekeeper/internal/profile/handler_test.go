package profile

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/auth"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/logger"
)

type fakeService struct {
	resp *MeResponse
	err  error
}

func (f *fakeService) GetMe(ctx context.Context, userID uuid.UUID) (*MeResponse, error) {
	return f.resp, f.err
}

func (f *fakeService) PatchMe(ctx context.Context, userID uuid.UUID, req PatchMeRequest) (*MeResponse, error) {
	return f.resp, f.err
}

func withAuthenticatedUser(r *http.Request, userID uuid.UUID) *http.Request {
	ctx := context.WithValue(r.Context(), auth.UserIDKey, userID.String())
	return r.WithContext(ctx)
}

func TestHandler_Me_Unauthenticated(t *testing.T) {
	h := NewHandler(&fakeService{}, logger.New(logger.LevelNone))

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	rec := httptest.NewRecorder()

	h.Me(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body["error"] != "UNAUTHORIZED" {
		t.Errorf("expected error UNAUTHORIZED, got %v", body["error"])
	}
}

func TestHandler_Me_Success(t *testing.T) {
	userID := uuid.New()
	resp := &MeResponse{Tier: "free"}
	h := NewHandler(&fakeService{resp: resp}, logger.New(logger.LevelNone))

	req := withAuthenticatedUser(httptest.NewRequest(http.MethodGet, "/me", nil), userID)
	rec := httptest.NewRecorder()

	h.Me(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandler_PatchMe_InvalidCountry(t *testing.T) {
	userID := uuid.New()
	h := NewHandler(&fakeService{}, logger.New(logger.LevelNone))

	body, _ := json.Marshal(map[string]string{"country": "us"})
	req := withAuthenticatedUser(httptest.NewRequest(http.MethodPatch, "/me", bytes.NewReader(body)), userID)
	rec := httptest.NewRecorder()

	h.PatchMe(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for lowercase country code, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandler_PatchMe_InvalidBirthYear(t *testing.T) {
	userID := uuid.New()
	h := NewHandler(&fakeService{}, logger.New(logger.LevelNone))

	body, _ := json.Marshal(map[string]int{"birth_year": 1800})
	req := withAuthenticatedUser(httptest.NewRequest(http.MethodPatch, "/me", bytes.NewReader(body)), userID)
	rec := httptest.NewRecorder()

	h.PatchMe(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for out-of-range birth year, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandler_PatchMe_ValidPartialPatch(t *testing.T) {
	userID := uuid.New()
	resp := &MeResponse{Tier: "free"}
	h := NewHandler(&fakeService{resp: resp}, logger.New(logger.LevelNone))

	body, _ := json.Marshal(map[string]string{"display_name": "New Name"})
	req := withAuthenticatedUser(httptest.NewRequest(http.MethodPatch, "/me", bytes.NewReader(body)), userID)
	rec := httptest.NewRecorder()

	h.PatchMe(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}
