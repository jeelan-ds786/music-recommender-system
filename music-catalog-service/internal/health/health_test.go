package health

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLive(t *testing.T) {
	handler := NewHandler(time.Second)
	request := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	response := httptest.NewRecorder()

	handler.Live(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
}

func TestReadyFailsWhenPostgresIsUnavailable(t *testing.T) {
	handler := NewHandler(50*time.Millisecond, Check{
		Name: "postgres",
		Ping: func(context.Context) error { return errors.New("unavailable") },
	})
	request := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	response := httptest.NewRecorder()

	handler.Ready(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
}
