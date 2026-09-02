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
	// Shutdown only waits for listeners it had already registered by the
	// time it was called. If ctx is canceled essentially immediately (as in
	// TestServeShutsDownHTTPAndGRPC), httpServer.Serve(httpListener)'s
	// goroutine may not have reached that bookkeeping yet — Shutdown then
	// returns without closing anything, and the listener is closed later,
	// asynchronously, by that goroutine's own early-return path once it
	// finally runs. Close it again here so callers can rely on it being
	// closed the moment serve() returns, not "eventually". Safe to call
	// twice: Close on an already-closed listener just errors, which we
	// ignore — it changes nothing we'd otherwise report.
	_ = httpListener.Close()

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
	// Same race as above, for the same reason, in grpc-go's Server.Serve /
	// GracefulStop (s.serveWG.Add(1) similarly only happens after the
	// already-stopped check, so GracefulStop's s.serveWG.Wait() can miss a
	// late-scheduled Serve goroutine too).
	_ = grpcListener.Close()

	if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) && !errors.Is(serveErr, grpc.ErrServerStopped) {
		shutdownErr = errors.Join(shutdownErr, serveErr)
	}
	return shutdownErr
}
