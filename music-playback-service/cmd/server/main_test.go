package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jeelan-ds786/music-recommender-system/music-playback-service/internal/logger"
)

type stubDatabase struct {
	err error
}

func (database stubDatabase) Ping(context.Context) error {
	return database.err
}

func TestLiveDoesNotCheckPostgres(t *testing.T) {
	router := newRouter(stubDatabase{err: errors.New("database unavailable")}, nil, "test-secret", logger.New(logger.LevelNone))
	request := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
}

func TestReadyReportsPostgresFailure(t *testing.T) {
	router := newRouter(stubDatabase{err: errors.New("database unavailable")}, nil, "test-secret", logger.New(logger.LevelNone))
	request := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
}

func TestServeShutsDownWhenContextIsCanceled(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	listener.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	server := &http.Server{
		Addr:              listener.Addr().String(),
		Handler:           http.NewServeMux(),
		ReadHeaderTimeout: time.Second,
	}

	if err := serve(ctx, server); err != nil {
		t.Fatalf("serve() error = %v", err)
	}
}
