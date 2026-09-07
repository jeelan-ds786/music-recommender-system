package health

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLiveIsProcessOnly(t *testing.T) {
	called := false
	handler := NewHandler(time.Second, Check{
		Name: "postgres",
		Ping: func(context.Context) error {
			called = true
			return errors.New("unavailable")
		},
	})
	request := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	responseRecorder := httptest.NewRecorder()

	handler.Live(responseRecorder, request)

	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", responseRecorder.Code, http.StatusOK)
	}
	if called {
		t.Fatal("liveness checked postgres")
	}
}

func TestReadyFailsWhenPostgresIsUnavailable(t *testing.T) {
	handler := NewHandler(time.Second, Check{
		Name: "postgres",
		Ping: func(context.Context) error { return errors.New("unavailable") },
	})
	request := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	responseRecorder := httptest.NewRecorder()

	handler.Ready(responseRecorder, request)

	if responseRecorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", responseRecorder.Code, http.StatusServiceUnavailable)
	}
}
