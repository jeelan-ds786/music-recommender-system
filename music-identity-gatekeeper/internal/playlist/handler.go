package playlist

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/auth"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/logger"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/reqid"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/response"
)

type Handler struct {
	service Service
	log     *logger.Logger
}

func NewHandler(service Service, log *logger.Logger) *Handler {
	return &Handler{
		service: service,
		log:     log,
	}
}

func (h *Handler) Create(
	w http.ResponseWriter,
	r *http.Request,
) {
	rid, _ := reqid.FromContext(r.Context())

	userID, ok := h.userIDFromRequest(w, r)
	if !ok {
		return
	}

	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error(rid, "Create rejected: invalid request body: %v", err)
		response.Error(w, http.StatusBadRequest, "INVALID_REQUEST_BODY")
		return
	}

	if validationErr := ValidateStruct(req); validationErr != nil {
		h.log.Error(rid, "Create rejected: validation failed for field=%s code=%s", validationErr.Field, validationErr.Error)
		response.ValidationError(w, http.StatusBadRequest, validationErr.Error, validationErr.Field)
		return
	}

	playlist, err := h.service.Create(r.Context(), userID, req)
	if err != nil {
		h.writeServiceError(w, rid, err)
		return
	}

	response.JSON(w, http.StatusCreated, playlist)
}

func (h *Handler) List(
	w http.ResponseWriter,
	r *http.Request,
) {
	rid, _ := reqid.FromContext(r.Context())

	userID, ok := h.userIDFromRequest(w, r)
	if !ok {
		return
	}

	playlists, err := h.service.List(r.Context(), userID)
	if err != nil {
		h.writeServiceError(w, rid, err)
		return
	}

	response.JSON(w, http.StatusOK, playlists)
}

func (h *Handler) Get(
	w http.ResponseWriter,
	r *http.Request,
) {
	rid, _ := reqid.FromContext(r.Context())

	userID, ok := h.userIDFromRequest(w, r)
	if !ok {
		return
	}

	playlistID, ok := h.uuidPathParam(w, r, "playlistID", "INVALID_PLAYLIST_ID")
	if !ok {
		return
	}

	playlist, err := h.service.Get(r.Context(), userID, playlistID)
	if err != nil {
		h.writeServiceError(w, rid, err)
		return
	}

	response.JSON(w, http.StatusOK, playlist)
}

func (h *Handler) Patch(
	w http.ResponseWriter,
	r *http.Request,
) {
	rid, _ := reqid.FromContext(r.Context())

	userID, ok := h.userIDFromRequest(w, r)
	if !ok {
		return
	}

	playlistID, ok := h.uuidPathParam(w, r, "playlistID", "INVALID_PLAYLIST_ID")
	if !ok {
		return
	}

	var req PatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error(rid, "Patch rejected: invalid request body: %v", err)
		response.Error(w, http.StatusBadRequest, "INVALID_REQUEST_BODY")
		return
	}

	if validationErr := ValidateStruct(req); validationErr != nil {
		h.log.Error(rid, "Patch rejected: validation failed for field=%s code=%s", validationErr.Field, validationErr.Error)
		response.ValidationError(w, http.StatusBadRequest, validationErr.Error, validationErr.Field)
		return
	}

	playlist, err := h.service.Patch(r.Context(), userID, playlistID, req)
	if err != nil {
		h.writeServiceError(w, rid, err)
		return
	}

	response.JSON(w, http.StatusOK, playlist)
}

func (h *Handler) Delete(
	w http.ResponseWriter,
	r *http.Request,
) {
	rid, _ := reqid.FromContext(r.Context())

	userID, ok := h.userIDFromRequest(w, r)
	if !ok {
		return
	}

	playlistID, ok := h.uuidPathParam(w, r, "playlistID", "INVALID_PLAYLIST_ID")
	if !ok {
		return
	}

	if err := h.service.Delete(r.Context(), userID, playlistID); err != nil {
		h.writeServiceError(w, rid, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *Handler) AddSong(
	w http.ResponseWriter,
	r *http.Request,
) {
	rid, _ := reqid.FromContext(r.Context())

	userID, ok := h.userIDFromRequest(w, r)
	if !ok {
		return
	}

	playlistID, ok := h.uuidPathParam(w, r, "playlistID", "INVALID_PLAYLIST_ID")
	if !ok {
		return
	}

	songID, ok := h.uuidPathParam(w, r, "songID", "INVALID_SONG_ID")
	if !ok {
		return
	}

	if err := h.service.AddSong(r.Context(), userID, playlistID, songID); err != nil {
		h.writeServiceError(w, rid, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"status": "added"})
}

func (h *Handler) RemoveSong(
	w http.ResponseWriter,
	r *http.Request,
) {
	rid, _ := reqid.FromContext(r.Context())

	userID, ok := h.userIDFromRequest(w, r)
	if !ok {
		return
	}

	playlistID, ok := h.uuidPathParam(w, r, "playlistID", "INVALID_PLAYLIST_ID")
	if !ok {
		return
	}

	songID, ok := h.uuidPathParam(w, r, "songID", "INVALID_SONG_ID")
	if !ok {
		return
	}

	if err := h.service.RemoveSong(r.Context(), userID, playlistID, songID); err != nil {
		h.writeServiceError(w, rid, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"status": "removed"})
}

// userIDFromRequest mirrors preference.Handler's helper of the same name.
// Duplicated rather than shared to avoid a cross-package import cycle, same
// reason preference doesn't share profile's.
func (h *Handler) userIDFromRequest(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	rid, _ := reqid.FromContext(r.Context())

	idStr, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		h.log.Error(rid, "request rejected: missing authenticated user in context")
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED")
		return uuid.Nil, false
	}

	userID, err := uuid.Parse(idStr)
	if err != nil {
		h.log.Error(rid, "request rejected: user id in context is not a valid UUID: %v", err)
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED")
		return uuid.Nil, false
	}

	return userID, true
}

func (h *Handler) uuidPathParam(w http.ResponseWriter, r *http.Request, name string, errCode string) (uuid.UUID, bool) {
	rid, _ := reqid.FromContext(r.Context())

	id, err := uuid.Parse(chi.URLParam(r, name))
	if err != nil {
		h.log.Error(rid, "request rejected: invalid %s: %v", name, err)
		response.Error(w, http.StatusBadRequest, errCode)
		return uuid.Nil, false
	}

	return id, true
}

func (h *Handler) writeServiceError(w http.ResponseWriter, rid string, err error) {
	switch {
	case errors.Is(err, ErrPlaylistNotFound):
		response.Error(w, http.StatusNotFound, "PLAYLIST_NOT_FOUND")
	case errors.Is(err, ErrSongNotInPlaylist):
		response.Error(w, http.StatusNotFound, "SONG_NOT_IN_PLAYLIST")
	default:
		h.log.Error(rid, "request failed: %v", err)
		response.Error(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR")
	}
}
