package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jeelan-ds786/music-recommender-system/music-playback-service/internal/db"
	"github.com/jeelan-ds786/music-recommender-system/music-playback-service/internal/health"
)

const shutdownTimeout = 10 * time.Second

type databasePinger interface {
	Ping(context.Context) error
}

func main() {
	ctx := context.Background()
	dsn := os.Getenv("DB_URL")
	if dsn == "" {
		log.Fatal("DB_URL is required")
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}

	pool, err := db.NewPostgresPool(ctx, dsn, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           newRouter(pool),
		ReadHeaderTimeout: 5 * time.Second,
	}

	shutdownContext, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Printf("playback HTTP server listening on :%s", port)
	if err := serve(shutdownContext, server); err != nil {
		log.Fatal(err)
	}
}

func newRouter(database databasePinger) http.Handler {
	router := chi.NewRouter()
	healthHandler := health.NewHandler(2*time.Second, health.Check{Name: "postgres", Ping: database.Ping})

	router.Get("/health/live", healthHandler.Live)
	router.Get("/health/ready", healthHandler.Ready)

	return router
}

func serve(ctx context.Context, server *http.Server) error {
	serveErrors := make(chan error, 1)
	go func() {
		serveErrors <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
	case err := <-serveErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	}

	shutdownContext, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	return server.Shutdown(shutdownContext)
}

var _ databasePinger = (*pgxpool.Pool)(nil)
