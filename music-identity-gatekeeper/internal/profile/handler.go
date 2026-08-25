package profile

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/auth"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/logger"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/reqid"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/response"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/user"
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

func (h *Handler) Me(
	w http.ResponseWriter,
	r *http.Request,
) {
	rid, _ := reqid.FromContext(r.Context())

	userID, ok := h.userIDFromRequest(w, r)
	if !ok {
		return
	}

	resp, err := h.service.GetMe(r.Context(), userID)
	if err != nil {
		h.writeServiceError(w, rid, err)
		return
	}

	response.JSON(w, http.StatusOK, resp)
}

func (h *Handler) PatchMe(
	w http.ResponseWriter,
	r *http.Request,
) {
	rid, _ := reqid.FromContext(r.Context())

	userID, ok := h.userIDFromRequest(w, r)
	if !ok {
		return
	}

	var req PatchMeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error(rid, "PatchMe rejected: invalid request body: %v", err)
		response.Error(w, http.StatusBadRequest, "INVALID_REQUEST_BODY")
		return
	}

	if validationErr := ValidateStruct(req); validationErr != nil {
		h.log.Error(rid, "PatchMe rejected: validation failed for field=%s code=%s", validationErr.Field, validationErr.Error)
		response.ValidationError(w, http.StatusBadRequest, validationErr.Error, validationErr.Field)
		return
	}

	if validationErr := ValidateBirthYear(req.BirthYear); validationErr != nil {
		h.log.Error(rid, "PatchMe rejected: validation failed for field=%s code=%s value=%d", validationErr.Field, validationErr.Error, *req.BirthYear)
		response.ValidationError(w, http.StatusBadRequest, validationErr.Error, validationErr.Field)
		return
	}

	resp, err := h.service.PatchMe(r.Context(), userID, req)
	if err != nil {
		h.writeServiceError(w, rid, err)
		return
	}

	response.JSON(w, http.StatusOK, resp)
}

// userIDFromRequest pulls the authenticated user id out of context (set by
// auth.AuthMiddleware) and parses it. A failure here means the caller
// wasn't authenticated at all, or the middleware let through something it
// shouldn't have — either way it's a 401, not a panic.
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

func (h *Handler) writeServiceError(w http.ResponseWriter, rid string, err error) {
	if errors.Is(err, user.ErrUserNotFound) {
		h.log.Error(rid, "request rejected: user not found: %v", err)
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED")
		return
	}

	h.log.Error(rid, "request failed: %v", err)
	response.Error(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR")
}
