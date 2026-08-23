package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/token"
)

type stubAuthService struct {
	registerResponse *RegisterResponse
	registerErr      error
	loginResponse    *token.TokenPair
	loginErr         error
	registerCalls    int
	loginCalls       int
}

func (s *stubAuthService) Register(context.Context, RegisterRequest) (*RegisterResponse, error) {
	s.registerCalls++
	return s.registerResponse, s.registerErr
}

func (s *stubAuthService) Login(context.Context, LoginRequest) (*token.TokenPair, error) {
	s.loginCalls++
	return s.loginResponse, s.loginErr
}

func (s *stubAuthService) Refresh(context.Context, RefreshRequest) (*token.TokenPair, error) {
	return nil, errors.New("not implemented")
}

func TestRegister(t *testing.T) {
	userID := uuid.New()
	tests := []struct {
		name          string
		body          string
		service       *stubAuthService
		wantStatus    int
		wantError     string
		wantField     string
		wantCallCount int
	}{
		{
			name: "successful registration",
			body: `{"email":"listener@example.com","password":"strong-password"}`,
			service: &stubAuthService{registerResponse: &RegisterResponse{
				ID: userID, Email: "listener@example.com", Message: "user registered successfully",
			}},
			wantStatus: http.StatusCreated, wantCallCount: 1,
		},
		{
			name:       "duplicate email",
			body:       `{"email":"listener@example.com","password":"strong-password"}`,
			service:    &stubAuthService{registerErr: ErrEmailAlreadyExists},
			wantStatus: http.StatusConflict, wantError: "EMAIL_ALREADY_EXISTS", wantCallCount: 1,
		},
		{
			name:       "invalid email",
			body:       `{"email":"not-an-email","password":"strong-password"}`,
			service:    &stubAuthService{},
			wantStatus: http.StatusBadRequest, wantError: "EMAIL_INVALID", wantField: "email",
		},
		{
			name:       "weak password",
			body:       `{"email":"listener@example.com","password":"short"}`,
			service:    &stubAuthService{},
			wantStatus: http.StatusBadRequest, wantError: "PASSWORD_TOO_SHORT", wantField: "password",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := NewHandler(test.service)
			request := httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader(test.body))
			response := httptest.NewRecorder()

			handler.Register(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
			if test.service.registerCalls != test.wantCallCount {
				t.Fatalf("service calls = %d, want %d", test.service.registerCalls, test.wantCallCount)
			}
			assertErrorResponse(t, response, test.wantError, test.wantField)
		})
	}
}

func TestLogin(t *testing.T) {
	tests := []struct {
		name       string
		service    *stubAuthService
		wantStatus int
		wantError  string
	}{
		{
			name: "successful login",
			service: &stubAuthService{loginResponse: &token.TokenPair{
				AccessToken: "access-token", RefreshToken: "refresh-token",
			}},
			wantStatus: http.StatusOK,
		},
		{
			name:       "wrong password",
			service:    &stubAuthService{loginErr: ErrInvalidCredentials},
			wantStatus: http.StatusUnauthorized, wantError: "INVALID_CREDENTIALS",
		},
		{
			name:       "unknown email",
			service:    &stubAuthService{loginErr: ErrInvalidCredentials},
			wantStatus: http.StatusUnauthorized, wantError: "INVALID_CREDENTIALS",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := NewHandler(test.service)
			request := httptest.NewRequest(
				http.MethodPost,
				"/auth/login",
				strings.NewReader(`{"email":"listener@example.com","password":"password"}`),
			)
			response := httptest.NewRecorder()

			handler.Login(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
			if test.service.loginCalls != 1 {
				t.Fatalf("service calls = %d, want 1", test.service.loginCalls)
			}
			assertErrorResponse(t, response, test.wantError, "")
		})
	}
}

func assertErrorResponse(t *testing.T, response *httptest.ResponseRecorder, wantError, wantField string) {
	t.Helper()
	if wantError == "" {
		return
	}

	var body struct {
		Error string `json:"error"`
		Field string `json:"field"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Error != wantError || body.Field != wantField {
		t.Fatalf("error response = %#v, want error=%q field=%q", body, wantError, wantField)
	}
}
