package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type stubDatabase struct {
	err error
}

func (database stubDatabase) Ping(context.Context) error {
	return database.err
}

func TestHealthRoutesArePublic(t *testing.T) {
	router := newRouter(stubDatabase{}, "admin-key")

	for _, path := range []string{"/health/live", "/health/ready"} {
		t.Run(path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, path, nil)
			responseRecorder := httptest.NewRecorder()

			router.ServeHTTP(responseRecorder, request)

			if responseRecorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", responseRecorder.Code, http.StatusOK)
			}
		})
	}
}

func TestReadyReportsDatabaseFailure(t *testing.T) {
	router := newRouter(stubDatabase{err: errors.New("database unavailable")}, "admin-key")
	request := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	responseRecorder := httptest.NewRecorder()

	router.ServeHTTP(responseRecorder, request)

	if responseRecorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", responseRecorder.Code, http.StatusServiceUnavailable)
	}
}
