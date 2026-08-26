package preference

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/auth"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/logger"
)

type fakeService struct {
	prefResp *PreferenceResponse
	page     *LikedSongsPage
	err      error
}

func (f *fakeService) GetPreferences(ctx context.Context, userID uuid.UUID) (*PreferenceResponse, error) {
	return f.prefResp, f.err
}

func (f *fakeService) Onboard(ctx context.Context, userID uuid.UUID, req OnboardingRequest) error {
	return f.err
}

func (f *fakeService) LikeSong(ctx context.Context, userID, songID uuid.UUID) error {
	return f.err
}

func (f *fakeService) UnlikeSong(ctx context.Context, userID, songID uuid.UUID) error {
	return f.err
}

func (f *fakeService) ListLikedSongs(ctx context.Context, userID uuid.UUID, cursor string, limit int) (*LikedSongsPage, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.page, nil
}

func (f *fakeService) FollowArtist(ctx context.Context, userID, artistID uuid.UUID) error {
	return f.err
}

func (f *fakeService) UnfollowArtist(ctx context.Context, userID, artistID uuid.UUID) error {
	return f.err
}

func withAuthenticatedUser(r *http.Request, userID uuid.UUID) *http.Request {
	ctx := context.WithValue(r.Context(), auth.UserIDKey, userID.String())
	return r.WithContext(ctx)
}

// router builds a minimal chi router so path-param handlers (LikeSong,
// UnlikeSong, FollowArtist, UnfollowArtist) can resolve chi.URLParam the
// same way they do when mounted from cmd/server/main.go.
func router(h *Handler) chi.Router {
	r := chi.NewRouter()
	r.Post("/me/likes/songs/{songID}", h.LikeSong)
	r.Delete("/me/likes/songs/{songID}", h.UnlikeSong)
	r.Post("/me/following/artists/{artistID}", h.FollowArtist)
	r.Delete("/me/following/artists/{artistID}", h.UnfollowArtist)
	return r
}

func TestHandler_LikeSong_Unauthenticated(t *testing.T) {
	h := NewHandler(&fakeService{}, logger.New(logger.LevelNone))

	req := httptest.NewRequest(http.MethodPost, "/me/likes/songs/"+uuid.New().String(), nil)
	rec := httptest.NewRecorder()

	router(h).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestHandler_LikeSong_InvalidUUID(t *testing.T) {
	h := NewHandler(&fakeService{}, logger.New(logger.LevelNone))

	req := httptest.NewRequest(http.MethodPost, "/me/likes/songs/not-a-uuid", nil)
	req = withAuthenticatedUser(req, uuid.New())
	rec := httptest.NewRecorder()

	router(h).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body["error"] != "INVALID_SONG_ID" {
		t.Errorf("expected error INVALID_SONG_ID, got %v", body["error"])
	}
}

func TestHandler_UnlikeSong_NotFound(t *testing.T) {
	h := NewHandler(&fakeService{err: ErrLikeNotFound}, logger.New(logger.LevelNone))

	req := httptest.NewRequest(http.MethodDelete, "/me/likes/songs/"+uuid.New().String(), nil)
	req = withAuthenticatedUser(req, uuid.New())
	rec := httptest.NewRecorder()

	router(h).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body["error"] != "LIKE_NOT_FOUND" {
		t.Errorf("expected error LIKE_NOT_FOUND, got %v", body["error"])
	}
}

func TestHandler_UnfollowArtist_NotFound(t *testing.T) {
	h := NewHandler(&fakeService{err: ErrFollowNotFound}, logger.New(logger.LevelNone))

	req := httptest.NewRequest(http.MethodDelete, "/me/following/artists/"+uuid.New().String(), nil)
	req = withAuthenticatedUser(req, uuid.New())
	rec := httptest.NewRecorder()

	router(h).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestHandler_FollowArtist_InvalidUUID(t *testing.T) {
	h := NewHandler(&fakeService{}, logger.New(logger.LevelNone))

	req := httptest.NewRequest(http.MethodPost, "/me/following/artists/not-a-uuid", nil)
	req = withAuthenticatedUser(req, uuid.New())
	rec := httptest.NewRecorder()

	router(h).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body["error"] != "INVALID_ARTIST_ID" {
		t.Errorf("expected error INVALID_ARTIST_ID, got %v", body["error"])
	}
}

func TestHandler_Onboarding_AlreadyCompleted(t *testing.T) {
	h := NewHandler(&fakeService{err: ErrOnboardingAlreadyCompleted}, logger.New(logger.LevelNone))

	body, _ := json.Marshal(OnboardingRequest{GenreSeeds: []string{"pop"}})
	req := httptest.NewRequest(http.MethodPost, "/me/onboarding", bytes.NewReader(body))
	req = withAuthenticatedUser(req, uuid.New())
	rec := httptest.NewRecorder()

	h.Onboarding(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "ONBOARDING_ALREADY_COMPLETED" {
		t.Errorf("expected error ONBOARDING_ALREADY_COMPLETED, got %v", resp["error"])
	}
}

func TestHandler_Onboarding_TooManyGenreSeeds(t *testing.T) {
	h := NewHandler(&fakeService{}, logger.New(logger.LevelNone))

	body, _ := json.Marshal(OnboardingRequest{
		GenreSeeds: []string{"pop", "rock", "jazz", "blues", "folk", "metal"},
	})
	req := httptest.NewRequest(http.MethodPost, "/me/onboarding", bytes.NewReader(body))
	req = withAuthenticatedUser(req, uuid.New())
	rec := httptest.NewRecorder()

	h.Onboarding(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandler_ListLikedSongs_Success(t *testing.T) {
	next := "some-cursor"
	page := &LikedSongsPage{
		Items:      []LikedSongItem{{SongID: uuid.New()}},
		NextCursor: &next,
	}
	h := NewHandler(&fakeService{page: page}, logger.New(logger.LevelNone))

	req := httptest.NewRequest(http.MethodGet, "/me/likes/songs?limit=1", nil)
	req = withAuthenticatedUser(req, uuid.New())
	rec := httptest.NewRecorder()

	h.ListLikedSongs(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp struct {
		Data LikedSongsPage `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Data.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(resp.Data.Items))
	}
	if resp.Data.NextCursor == nil || *resp.Data.NextCursor != next {
		t.Errorf("expected next_cursor %q, got %v", next, resp.Data.NextCursor)
	}
}

func TestHandler_ListLikedSongs_InvalidLimit(t *testing.T) {
	h := NewHandler(&fakeService{}, logger.New(logger.LevelNone))

	req := httptest.NewRequest(http.MethodGet, "/me/likes/songs?limit=not-a-number", nil)
	req = withAuthenticatedUser(req, uuid.New())
	rec := httptest.NewRecorder()

	h.ListLikedSongs(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}
