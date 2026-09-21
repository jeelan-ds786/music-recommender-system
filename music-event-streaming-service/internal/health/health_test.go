package health

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestReadyReportsFailedDependency(t *testing.T) {
	handler := New(time.Second, Check{Name: "kafka", Ping: func(context.Context) error { return errors.New("unavailable") }})
	response := httptest.NewRecorder()
	handler.Ready(response, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
}

func TestLiveDoesNotCheckDependencies(t *testing.T) {
	handler := New(time.Second, Check{Name: "kafka", Ping: func(context.Context) error { return errors.New("unavailable") }})
	response := httptest.NewRecorder()
	handler.Live(response, httptest.NewRequest(http.MethodGet, "/health/live", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
}
