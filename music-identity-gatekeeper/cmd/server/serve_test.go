package main

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"google.golang.org/grpc"
)

func TestServeShutsDownHTTPAndGRPC(t *testing.T) {
	httpListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("HTTP listen: %v", err)
	}
	grpcListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("gRPC listen: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- serve(
			ctx,
			&http.Server{Handler: http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})},
			httpListener,
			grpc.NewServer(),
			grpcListener,
		)
	}()

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serve() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("servers did not shut down")
	}

	assertListenerClosed(t, httpListener.Addr().String())
	assertListenerClosed(t, grpcListener.Addr().String())
}

func assertListenerClosed(t *testing.T, address string) {
	t.Helper()
	connection, err := net.DialTimeout("tcp", address, 100*time.Millisecond)
	if err == nil {
		_ = connection.Close()
		t.Fatalf("listener %s still accepts connections", address)
	}
}
