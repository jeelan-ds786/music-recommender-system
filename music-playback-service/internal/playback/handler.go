package playback

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	playbackauth "github.com/jeelan-ds786/music-recommender-system/music-playback-service/internal/auth"
	"github.com/jeelan-ds786/music-recommender-system/music-playback-service/internal/logger"
	"github.com/jeelan-ds786/music-recommender-system/music-playback-service/internal/reqid"
	"github.com/jeelan-ds786/music-recommender-system/music-playback-service/internal/response"
)

type Handler struct {
	service Service
	log     *logger.Logger
}

func NewHandler(service Service, log *logger.Logger) *Handler {
	return &Handler{service: service, log: log}
}

func (h *Handler) Ingest(w http.ResponseWriter, r *http.Request) {
	rid, _ := reqid.FromContext(r.Context())

	userID, ok := h.userIDFromRequest(w, r)
	if !ok {
		return
	}

	var req IngestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error(rid, "Ingest rejected: invalid request body: %v", err)
		response.Error(w, http.StatusBadRequest, "INVALID_REQUEST_BODY")
		return
	}

	resp, validationErr, err := h.service.IngestSingle(r.Context(), userID, req)
	if err != nil {
		h.log.Error(rid, "Ingest failed for user_id=%s: %v", userID, err)
		response.Error(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR")
		return
	}
	if validationErr != nil {
		response.ValidationError(w, http.StatusBadRequest, validationErr.Error, validationErr.Field)
		return
	}

	response.JSON(w, http.StatusAccepted, resp)
}

func (h *Handler) IngestBatch(w http.ResponseWriter, r *http.Request) {
	rid, _ := reqid.FromContext(r.Context())

	userID, ok := h.userIDFromRequest(w, r)
	if !ok {
		return
	}

	var req BatchIngestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error(rid, "IngestBatch rejected: invalid request body: %v", err)
		response.Error(w, http.StatusBadRequest, "INVALID_REQUEST_BODY")
		return
	}

	results, validationErr, err := h.service.IngestBatch(r.Context(), userID, req)
	if err != nil {
		h.log.Error(rid, "IngestBatch failed for user_id=%s: %v", userID, err)
		response.Error(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR")
		return
	}
	if validationErr != nil {
		response.ValidationError(w, http.StatusBadRequest, validationErr.Error, validationErr.Field)
		return
	}

	response.JSON(w, http.StatusAccepted, BatchIngestResponse{Results: results})
}

func (h *Handler) GetSessionEvents(w http.ResponseWriter, r *http.Request) {
	rid, _ := reqid.FromContext(r.Context())

	userID, ok := h.userIDFromRequest(w, r)
	if !ok {
		return
	}

	sessionID, ok := h.uuidPathParam(w, r, "sessionID", "INVALID_SESSION_ID")
	if !ok {
		return
	}

	cursor := r.URL.Query().Get("cursor")

	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			h.log.Error(rid, "GetSessionEvents rejected: invalid limit=%q", raw)
			response.Error(w, http.StatusBadRequest, "INVALID_LIMIT")
			return
		}
		limit = parsed
	}

	page, err := h.service.GetSessionEvents(r.Context(), userID, sessionID, cursor, limit)
	if err != nil {
		switch {
		case errors.Is(err, ErrSessionNotFound):
			response.Error(w, http.StatusNotFound, "SESSION_NOT_FOUND")
		case errors.Is(err, ErrInvalidCursor):
			response.Error(w, http.StatusBadRequest, "INVALID_CURSOR")
		default:
			h.log.Error(rid, "GetSessionEvents failed for user_id=%s session_id=%s: %v", userID, sessionID, err)
			response.Error(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR")
		}
		return
	}

	response.JSON(w, http.StatusOK, page)
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

func (h *Handler) userIDFromRequest(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	rid, _ := reqid.FromContext(r.Context())

	idStr, ok := playbackauth.UserIDFromContext(r.Context())
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
