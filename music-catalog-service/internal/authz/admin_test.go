package authz

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAdminKeyMiddleware(t *testing.T) {
	tests := []struct {
		name       string
		configured string
		provided   string
		method     string
		wantStatus int
	}{
		{name: "valid key", configured: "test-admin-key", provided: "test-admin-key", wantStatus: http.StatusNoContent},
		{name: "missing key", configured: "test-admin-key", wantStatus: http.StatusUnauthorized},
		{name: "incorrect key", configured: "test-admin-key", provided: "wrong-key", wantStatus: http.StatusUnauthorized},
		{name: "empty configured key", wantStatus: http.StatusUnauthorized},
		{name: "public GET", configured: "test-admin-key", method: http.MethodGet, wantStatus: http.StatusNoContent},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			})
			handler := AdminKeyMiddleware(test.configured)(next)
			method := test.method
			if method == "" {
				method = http.MethodPost
			}
			request := httptest.NewRequest(method, "/artists", nil)
			if test.provided != "" {
				request.Header.Set(AdminKeyHeader, test.provided)
			}
			responseRecorder := httptest.NewRecorder()

			handler.ServeHTTP(responseRecorder, request)

			if responseRecorder.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", responseRecorder.Code, test.wantStatus)
			}
			if test.wantStatus == http.StatusUnauthorized && responseRecorder.Body.String() != "{\"error\":\"unauthorized\"}\n" {
				t.Fatalf("body = %q, want unauthorized error", responseRecorder.Body.String())
			}
		})
	}
}
