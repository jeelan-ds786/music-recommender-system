package preference

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

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

func (h *Handler) Onboarding(
	w http.ResponseWriter,
	r *http.Request,
) {
	rid, _ := reqid.FromContext(r.Context())

	userID, ok := h.userIDFromRequest(w, r)
	if !ok {
		return
	}

	var req OnboardingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error(rid, "Onboarding rejected: invalid request body: %v", err)
		response.Error(w, http.StatusBadRequest, "INVALID_REQUEST_BODY")
		return
	}

	if validationErr := ValidateStruct(req); validationErr != nil {
		h.log.Error(rid, "Onboarding rejected: validation failed for field=%s code=%s", validationErr.Field, validationErr.Error)
		response.ValidationError(w, http.StatusBadRequest, validationErr.Error, validationErr.Field)
		return
	}

	if err := h.service.Onboard(r.Context(), userID, req); err != nil {
		h.writeServiceError(w, rid, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"status": "onboarded"})
}

func (h *Handler) LikeSong(
	w http.ResponseWriter,
	r *http.Request,
) {
	rid, _ := reqid.FromContext(r.Context())

	userID, ok := h.userIDFromRequest(w, r)
	if !ok {
		return
	}

	songID, ok := h.uuidPathParam(w, r, "songID", "INVALID_SONG_ID")
	if !ok {
		return
	}

	if err := h.service.LikeSong(r.Context(), userID, songID); err != nil {
		h.writeServiceError(w, rid, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"status": "liked"})
}

func (h *Handler) UnlikeSong(
	w http.ResponseWriter,
	r *http.Request,
) {
	rid, _ := reqid.FromContext(r.Context())

	userID, ok := h.userIDFromRequest(w, r)
	if !ok {
		return
	}

	songID, ok := h.uuidPathParam(w, r, "songID", "INVALID_SONG_ID")
	if !ok {
		return
	}

	if err := h.service.UnlikeSong(r.Context(), userID, songID); err != nil {
		h.writeServiceError(w, rid, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"status": "unliked"})
}

func (h *Handler) ListLikedSongs(
	w http.ResponseWriter,
	r *http.Request,
) {
	rid, _ := reqid.FromContext(r.Context())

	userID, ok := h.userIDFromRequest(w, r)
	if !ok {
		return
	}

	cursor := r.URL.Query().Get("cursor")

	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			h.log.Error(rid, "ListLikedSongs rejected: invalid limit=%q", raw)
			response.Error(w, http.StatusBadRequest, "INVALID_LIMIT")
			return
		}
		limit = parsed
	}

	page, err := h.service.ListLikedSongs(r.Context(), userID, cursor, limit)
	if err != nil {
		h.writeServiceError(w, rid, err)
		return
	}

	response.JSON(w, http.StatusOK, page)
}

func (h *Handler) FollowArtist(
	w http.ResponseWriter,
	r *http.Request,
) {
	rid, _ := reqid.FromContext(r.Context())

	userID, ok := h.userIDFromRequest(w, r)
	if !ok {
		return
	}

	artistID, ok := h.uuidPathParam(w, r, "artistID", "INVALID_ARTIST_ID")
	if !ok {
		return
	}

	if err := h.service.FollowArtist(r.Context(), userID, artistID); err != nil {
		h.writeServiceError(w, rid, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"status": "followed"})
}

func (h *Handler) UnfollowArtist(
	w http.ResponseWriter,
	r *http.Request,
) {
	rid, _ := reqid.FromContext(r.Context())

	userID, ok := h.userIDFromRequest(w, r)
	if !ok {
		return
	}

	artistID, ok := h.uuidPathParam(w, r, "artistID", "INVALID_ARTIST_ID")
	if !ok {
		return
	}

	if err := h.service.UnfollowArtist(r.Context(), userID, artistID); err != nil {
		h.writeServiceError(w, rid, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"status": "unfollowed"})
}

// userIDFromRequest mirrors profile.Handler's helper of the same name.
// Duplicated rather than shared because internal/profile already imports
// internal/preference — importing back the other way would cycle.
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
	case errors.Is(err, ErrLikeNotFound):
		response.Error(w, http.StatusNotFound, "LIKE_NOT_FOUND")
	case errors.Is(err, ErrFollowNotFound):
		response.Error(w, http.StatusNotFound, "FOLLOW_NOT_FOUND")
	case errors.Is(err, ErrOnboardingAlreadyCompleted):
		response.Error(w, http.StatusConflict, "ONBOARDING_ALREADY_COMPLETED")
	case errors.Is(err, ErrInvalidCursor):
		response.Error(w, http.StatusBadRequest, "INVALID_CURSOR")
	default:
		h.log.Error(rid, "request failed: %v", err)
		response.Error(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR")
	}
}
