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

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jeelan-ds786/music-recommender-system/music-event-streaming-service/internal/config"
	"github.com/jeelan-ds786/music-recommender-system/music-event-streaming-service/internal/consumer"
	"github.com/jeelan-ds786/music-recommender-system/music-event-streaming-service/internal/contracts"
	"github.com/jeelan-ds786/music-recommender-system/music-event-streaming-service/internal/handler"
	"github.com/jeelan-ds786/music-recommender-system/music-event-streaming-service/internal/health"
	"github.com/jeelan-ds786/music-recommender-system/music-event-streaming-service/internal/idempotency"
)

const shutdownTimeout = 10 * time.Second

func main() {
	settings, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, settings.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		log.Fatal(err)
	}

	pinger := consumer.NewKafkaPinger(settings.Brokers)
	server := newHTTPServer(settings.Port, pool, pinger)
	serverErrors := make(chan error, 1)
	go func() {
		log.Printf("streaming HTTP server listening on :%s", settings.Port)
		serverErrors <- server.ListenAndServe()
	}()

	store := idempotency.NewStore(pool)
	decoder := contracts.Decoder{}
	workers := make([]*consumer.Consumer, 0, settings.Concurrency)
	for range settings.Concurrency {
		reader := consumer.NewKafkaReader(settings.Brokers, settings.Topics, settings.GroupID, decoder)
		workers = append(workers, consumer.New(reader, handler.NewValidation("contract-validation-v1"), store))
	}
	consumerErrors := make(chan error, 1)
	go func() { consumerErrors <- consumer.RunGroup(ctx, settings.DrainTimeout, workers...) }()

	consumerFinished := false
	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Printf("HTTP server failed: %v", err)
		}
		stop()
	case err := <-consumerErrors:
		consumerFinished = true
		if err != nil {
			log.Printf("consumer failed: %v", err)
		}
		stop()
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP shutdown failed: %v", err)
	}
	if !consumerFinished {
		if err := <-consumerErrors; err != nil {
			log.Printf("consumer shutdown failed: %v", err)
		}
	}
}

type pinger interface {
	Ping(context.Context) error
}

func newHTTPServer(port string, database, kafka pinger) *http.Server {
	healthHandler := health.New(2*time.Second,
		health.Check{Name: "postgres", Ping: database.Ping},
		health.Check{Name: "kafka", Ping: kafka.Ping},
	)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", healthHandler.Live)
	mux.HandleFunc("GET /health/ready", healthHandler.Ready)
	return &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}
