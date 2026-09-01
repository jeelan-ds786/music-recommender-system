package auth

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/token"
)

type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{
		service: service,
	}
}

func (h *Handler) Register(
	w http.ResponseWriter,
	r *http.Request,
) {
	var req RegisterRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(
			w,
			http.StatusBadRequest,
			"INVALID_REQUEST_BODY",
		)
		return
	}

	if validationErr := ValidateStruct(req); validationErr != nil {
		writeJSON(
			w,
			http.StatusBadRequest,
			validationErr,
		)
		return
	}

	response, err := h.service.Register(
		r.Context(),
		req,
	)

	if err != nil {

		if errors.Is(err, ErrEmailAlreadyExists) {
			writeError(
				w,
				http.StatusConflict,
				"EMAIL_ALREADY_EXISTS",
			)
			return
		}

		writeError(
			w,
			http.StatusInternalServerError,
			"INTERNAL_SERVER_ERROR",
		)
		return
	}

	writeJSON(
		w,
		http.StatusCreated,
		response,
	)
}

func (h *Handler) Login(
	w http.ResponseWriter,
	r *http.Request,
) {
	var req LoginRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(
			w,
			http.StatusBadRequest,
			"INVALID_REQUEST_BODY",
		)
		return
	}

	if validationErr := ValidateStruct(req); validationErr != nil {
		writeJSON(
			w,
			http.StatusBadRequest,
			validationErr,
		)
		return
	}

	tokenPair, err := h.service.Login(
		r.Context(),
		req,
	)

	if err != nil {

		if errors.Is(err, ErrInvalidCredentials) {
			writeError(
				w,
				http.StatusUnauthorized,
				"INVALID_CREDENTIALS",
			)
			return
		}

		writeError(
			w,
			http.StatusInternalServerError,
			"INTERNAL_SERVER_ERROR",
		)
		return
	}

	writeJSON(
		w,
		http.StatusOK,
		map[string]any{
			"access_token":  tokenPair.AccessToken,
			"refresh_token": tokenPair.RefreshToken,
		},
	)
}

func (h *Handler) Refresh(
	w http.ResponseWriter,
	r *http.Request,
) {
	var req RefreshRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(
			w,
			http.StatusBadRequest,
			"INVALID_REQUEST_BODY",
		)
		return
	}

	if validationErr := ValidateStruct(req); validationErr != nil {
		writeJSON(
			w,
			http.StatusBadRequest,
			validationErr,
		)
		return
	}

	tokenPair, err := h.service.Refresh(
		r.Context(),
		req,
	)

	if err != nil {
		writeError(
			w,
			http.StatusUnauthorized,
			"INVALID_REFRESH_TOKEN",
		)
		return
	}

	writeJSON(
		w,
		http.StatusOK,
		map[string]any{
			"access_token":  tokenPair.AccessToken,
			"refresh_token": tokenPair.RefreshToken,
		},
	)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	var req LogoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST_BODY")
		return
	}
	if validationErr := ValidateStruct(req); validationErr != nil {
		writeJSON(w, http.StatusBadRequest, validationErr)
		return
	}

	jti, jtiOK := JTIFromContext(r.Context())
	expiresAt, expiryOK := ExpiryFromContext(r.Context())
	userIDValue, userIDOK := UserIDFromContext(r.Context())
	userID, userIDErr := uuid.Parse(userIDValue)
	if !jtiOK || !expiryOK || !userIDOK || userIDErr != nil {
		writeError(w, http.StatusUnauthorized, "INVALID_ACCESS_TOKEN")
		return
	}

	if err := h.service.Logout(r.Context(), req, userID, jti, expiresAt); err != nil {
		if errors.Is(err, token.ErrRevocationUnavailable) {
			writeError(w, http.StatusServiceUnavailable, "AUTHORIZATION_UNAVAILABLE")
			return
		}
		writeError(w, http.StatusUnauthorized, "INVALID_REFRESH_TOKEN")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Me(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeError(
			w,
			http.StatusUnauthorized,
			"UNAUTHORIZED",
		)
		return
	}

	writeJSON(
		w,
		http.StatusOK,
		map[string]any{
			"user_id": userID,
		},
	)
}

func writeJSON(
	w http.ResponseWriter,
	status int,
	payload any,
) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(
	w http.ResponseWriter,
	status int,
	code string,
) {
	writeJSON(
		w,
		status,
		map[string]string{
			"error": code,
		},
	)
}
