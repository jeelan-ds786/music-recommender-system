package playlist

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
	playlistResp *PlaylistResponse
	listResp     []PlaylistResponse
	detailResp   *PlaylistDetailResponse
	err          error
}

func (f *fakeService) Create(ctx context.Context, userID uuid.UUID, req CreateRequest) (*PlaylistResponse, error) {
	return f.playlistResp, f.err
}

func (f *fakeService) List(ctx context.Context, userID uuid.UUID) ([]PlaylistResponse, error) {
	return f.listResp, f.err
}

func (f *fakeService) Get(ctx context.Context, userID, playlistID uuid.UUID) (*PlaylistDetailResponse, error) {
	return f.detailResp, f.err
}

func (f *fakeService) Patch(ctx context.Context, userID, playlistID uuid.UUID, req PatchRequest) (*PlaylistResponse, error) {
	return f.playlistResp, f.err
}

func (f *fakeService) Delete(ctx context.Context, userID, playlistID uuid.UUID) error {
	return f.err
}

func (f *fakeService) AddSong(ctx context.Context, userID, playlistID, songID uuid.UUID) error {
	return f.err
}

func (f *fakeService) RemoveSong(ctx context.Context, userID, playlistID, songID uuid.UUID) error {
	return f.err
}

func withAuthenticatedUser(r *http.Request, userID uuid.UUID) *http.Request {
	ctx := context.WithValue(r.Context(), auth.UserIDKey, userID.String())
	return r.WithContext(ctx)
}

// router builds a minimal chi router so path-param handlers can resolve
// chi.URLParam the same way they do when mounted from cmd/server/main.go.
func router(h *Handler) chi.Router {
	r := chi.NewRouter()
	r.Post("/me/playlists", h.Create)
	r.Get("/me/playlists", h.List)
	r.Get("/me/playlists/{playlistID}", h.Get)
	r.Patch("/me/playlists/{playlistID}", h.Patch)
	r.Delete("/me/playlists/{playlistID}", h.Delete)
	r.Post("/me/playlists/{playlistID}/songs/{songID}", h.AddSong)
	r.Delete("/me/playlists/{playlistID}/songs/{songID}", h.RemoveSong)
	return r
}

func TestHandler_Create_Unauthenticated(t *testing.T) {
	h := NewHandler(&fakeService{}, logger.New(logger.LevelNone))

	body, _ := json.Marshal(CreateRequest{Name: "Road Trip"})
	req := httptest.NewRequest(http.MethodPost, "/me/playlists", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	router(h).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestHandler_Create_MissingName(t *testing.T) {
	h := NewHandler(&fakeService{}, logger.New(logger.LevelNone))

	body, _ := json.Marshal(CreateRequest{})
	req := httptest.NewRequest(http.MethodPost, "/me/playlists", bytes.NewReader(body))
	req = withAuthenticatedUser(req, uuid.New())
	rec := httptest.NewRecorder()

	router(h).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "NAME_REQUIRED" {
		t.Errorf("expected error NAME_REQUIRED, got %v", resp["error"])
	}
}

func TestHandler_Create_Success(t *testing.T) {
	resp := &PlaylistResponse{ID: uuid.New(), Name: "Road Trip"}
	h := NewHandler(&fakeService{playlistResp: resp}, logger.New(logger.LevelNone))

	body, _ := json.Marshal(CreateRequest{Name: "Road Trip"})
	req := httptest.NewRequest(http.MethodPost, "/me/playlists", bytes.NewReader(body))
	req = withAuthenticatedUser(req, uuid.New())
	rec := httptest.NewRecorder()

	router(h).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
}

func TestHandler_Get_NotFound(t *testing.T) {
	h := NewHandler(&fakeService{err: ErrPlaylistNotFound}, logger.New(logger.LevelNone))

	req := httptest.NewRequest(http.MethodGet, "/me/playlists/"+uuid.New().String(), nil)
	req = withAuthenticatedUser(req, uuid.New())
	rec := httptest.NewRecorder()

	router(h).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "PLAYLIST_NOT_FOUND" {
		t.Errorf("expected error PLAYLIST_NOT_FOUND, got %v", resp["error"])
	}
}

func TestHandler_Get_InvalidPlaylistID(t *testing.T) {
	h := NewHandler(&fakeService{}, logger.New(logger.LevelNone))

	req := httptest.NewRequest(http.MethodGet, "/me/playlists/not-a-uuid", nil)
	req = withAuthenticatedUser(req, uuid.New())
	rec := httptest.NewRecorder()

	router(h).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "INVALID_PLAYLIST_ID" {
		t.Errorf("expected error INVALID_PLAYLIST_ID, got %v", resp["error"])
	}
}

func TestHandler_List_Success(t *testing.T) {
	list := []PlaylistResponse{{ID: uuid.New(), Name: "A"}}
	h := NewHandler(&fakeService{listResp: list}, logger.New(logger.LevelNone))

	req := httptest.NewRequest(http.MethodGet, "/me/playlists", nil)
	req = withAuthenticatedUser(req, uuid.New())
	rec := httptest.NewRecorder()

	router(h).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp struct {
		Data []PlaylistResponse `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 playlist, got %d", len(resp.Data))
	}
}

func TestHandler_Patch_NotFound(t *testing.T) {
	h := NewHandler(&fakeService{err: ErrPlaylistNotFound}, logger.New(logger.LevelNone))

	newName := "Renamed"
	body, _ := json.Marshal(PatchRequest{Name: &newName})
	req := httptest.NewRequest(http.MethodPatch, "/me/playlists/"+uuid.New().String(), bytes.NewReader(body))
	req = withAuthenticatedUser(req, uuid.New())
	rec := httptest.NewRecorder()

	router(h).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestHandler_Delete_NotFound(t *testing.T) {
	h := NewHandler(&fakeService{err: ErrPlaylistNotFound}, logger.New(logger.LevelNone))

	req := httptest.NewRequest(http.MethodDelete, "/me/playlists/"+uuid.New().String(), nil)
	req = withAuthenticatedUser(req, uuid.New())
	rec := httptest.NewRecorder()

	router(h).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestHandler_AddSong_InvalidSongID(t *testing.T) {
	h := NewHandler(&fakeService{}, logger.New(logger.LevelNone))

	req := httptest.NewRequest(http.MethodPost, "/me/playlists/"+uuid.New().String()+"/songs/not-a-uuid", nil)
	req = withAuthenticatedUser(req, uuid.New())
	rec := httptest.NewRecorder()

	router(h).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "INVALID_SONG_ID" {
		t.Errorf("expected error INVALID_SONG_ID, got %v", resp["error"])
	}
}

func TestHandler_AddSong_Success(t *testing.T) {
	h := NewHandler(&fakeService{}, logger.New(logger.LevelNone))

	req := httptest.NewRequest(http.MethodPost, "/me/playlists/"+uuid.New().String()+"/songs/"+uuid.New().String(), nil)
	req = withAuthenticatedUser(req, uuid.New())
	rec := httptest.NewRecorder()

	router(h).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestHandler_RemoveSong_NotFound(t *testing.T) {
	h := NewHandler(&fakeService{err: ErrSongNotInPlaylist}, logger.New(logger.LevelNone))

	req := httptest.NewRequest(http.MethodDelete, "/me/playlists/"+uuid.New().String()+"/songs/"+uuid.New().String(), nil)
	req = withAuthenticatedUser(req, uuid.New())
	rec := httptest.NewRecorder()

	router(h).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "SONG_NOT_IN_PLAYLIST" {
		t.Errorf("expected error SONG_NOT_IN_PLAYLIST, got %v", resp["error"])
	}
}
