package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type stubPinger struct {
	err error
}

func (p stubPinger) Ping(context.Context) error { return p.err }

func TestReadinessFailsWhenKafkaIsUnavailable(t *testing.T) {
	server := newHTTPServer("0", stubPinger{}, stubPinger{err: errors.New("kafka unavailable")})
	request := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	response := httptest.NewRecorder()
	server.Handler.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
}
