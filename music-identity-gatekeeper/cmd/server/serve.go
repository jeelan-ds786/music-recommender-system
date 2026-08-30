package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	"google.golang.org/grpc"
)

const shutdownTimeout = 10 * time.Second

func serve(
	ctx context.Context,
	httpServer *http.Server,
	httpListener net.Listener,
	grpcServer *grpc.Server,
	grpcListener net.Listener,
) error {
	errCh := make(chan error, 2)
	go func() {
		errCh <- httpServer.Serve(httpListener)
	}()
	go func() {
		errCh <- grpcServer.Serve(grpcListener)
	}()

	var serveErr error
	select {
	case <-ctx.Done():
	case serveErr = <-errCh:
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	shutdownErr := httpServer.Shutdown(shutdownCtx)
	if shutdownErr != nil {
		shutdownErr = errors.Join(shutdownErr, httpServer.Close())
	}

	grpcStopped := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(grpcStopped)
	}()
	select {
	case <-grpcStopped:
	case <-shutdownCtx.Done():
		grpcServer.Stop()
		<-grpcStopped
		shutdownErr = errors.Join(shutdownErr, shutdownCtx.Err())
	}

	if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) && !errors.Is(serveErr, grpc.ErrServerStopped) {
		shutdownErr = errors.Join(shutdownErr, serveErr)
	}
	return shutdownErr
}
