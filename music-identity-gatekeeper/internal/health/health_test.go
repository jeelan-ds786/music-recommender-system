package health

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestReadyFailsWhenPostgresIsUnavailable(t *testing.T) {
	handler := NewHandler(50*time.Millisecond,
		Check{Name: "postgres", Ping: func(context.Context) error { return errors.New("unavailable") }},
		Check{Name: "redis", Ping: func(context.Context) error { return nil }},
	)
	request := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	response := httptest.NewRecorder()

	handler.Ready(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
	if got := response.Body.String(); got != "{\"failed\":[\"postgres\"],\"status\":\"unavailable\"}\n" {
		t.Fatalf("body = %q", got)
	}
}

func TestReadyUsesBoundedCheckTimeout(t *testing.T) {
	handler := NewHandler(10*time.Millisecond, Check{
		Name: "postgres",
		Ping: func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		},
	})
	request := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	response := httptest.NewRecorder()

	started := time.Now()
	handler.Ready(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
	if elapsed := time.Since(started); elapsed > 100*time.Millisecond {
		t.Fatalf("readiness took %s, want under 100ms", elapsed)
	}
}
