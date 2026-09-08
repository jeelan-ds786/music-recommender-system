package playback

import (
	"encoding/json"
	"net/http"

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
