package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jeelan-ds786/music-recommender-system/music-playback-service/internal/auth"
	"github.com/jeelan-ds786/music-recommender-system/music-playback-service/internal/db"
	playbackevent "github.com/jeelan-ds786/music-recommender-system/music-playback-service/internal/event"
	"github.com/jeelan-ds786/music-recommender-system/music-playback-service/internal/health"
	"github.com/jeelan-ds786/music-recommender-system/music-playback-service/internal/httplog"
	"github.com/jeelan-ds786/music-recommender-system/music-playback-service/internal/logger"
	"github.com/jeelan-ds786/music-recommender-system/music-playback-service/internal/playback"
	"github.com/jeelan-ds786/music-recommender-system/music-playback-service/internal/reqid"
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
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET is required")
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}

	appLogger := logger.New(logger.ParseLevel(os.Getenv("LOG_LEVEL")))

	pool, err := db.NewPostgresPool(ctx, dsn, db.NewQueryTracer(appLogger))
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	shutdownContext, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	serviceOptions := make([]playback.ServiceOption, 0, 1)
	if brokers := splitBrokers(os.Getenv("KAFKA_BROKERS")); len(brokers) > 0 {
		publisher := playbackevent.NewKafkaPublisher(brokers)
		defer func() {
			if closeErr := publisher.Close(); closeErr != nil {
				appLogger.Error("", "failed to close Kafka publisher: %v", closeErr)
			}
		}()

		outbox := playbackevent.NewOutbox(pool)
		serviceOptions = append(serviceOptions, playback.WithDirectPublisher(playbackevent.NewDirect(outbox, publisher)))
		if relayEnabled(os.Getenv("KAFKA_RELAY_ENABLED")) {
			relay := playbackevent.NewRelay(outbox, publisher, 50, 5)
			go relay.Run(shutdownContext, relayInterval(os.Getenv("KAFKA_RELAY_INTERVAL")))
		}
	}

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           newRouter(pool, pool, jwtSecret, appLogger, serviceOptions...),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("playback HTTP server listening on :%s", port)
	if err := serve(shutdownContext, server); err != nil {
		log.Fatal(err)
	}
}

// newRouter takes the health-check pinger and the real pool separately:
// tests exercise health checks against a stubDatabase that only implements
// Ping, while the playback repository needs actual query methods a stub
// doesn't provide. Pass nil for pool in tests that don't touch the
// playback routes.
func newRouter(database databasePinger, pool *pgxpool.Pool, jwtSecret string, appLogger *logger.Logger, serviceOptions ...playback.ServiceOption) http.Handler {
	router := chi.NewRouter()
	router.Use(reqid.Middleware)
	router.Use(httplog.Middleware(appLogger))

	healthHandler := health.NewHandler(2*time.Second, health.Check{Name: "postgres", Ping: database.Ping})

	router.Get("/health/live", healthHandler.Live)
	router.Get("/health/ready", healthHandler.Ready)

	playbackHandler := playback.NewHandler(playback.NewService(playback.NewRepository(pool), appLogger, serviceOptions...), appLogger)

	router.Group(func(r chi.Router) {
		r.Use(auth.Middleware(jwtSecret, appLogger))
		r.Post("/v1/playback/events", playbackHandler.Ingest)
		r.Post("/v1/playback/events:batch", playbackHandler.IngestBatch)
	})

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

func splitBrokers(value string) []string {
	parts := strings.Split(value, ",")
	brokers := make([]string, 0, len(parts))
	for _, part := range parts {
		if broker := strings.TrimSpace(part); broker != "" {
			brokers = append(brokers, broker)
		}
	}
	return brokers
}

func relayEnabled(value string) bool {
	if value == "" {
		return true
	}
	enabled, err := strconv.ParseBool(value)
	return err == nil && enabled
}

func relayInterval(value string) time.Duration {
	interval, err := time.ParseDuration(value)
	if err != nil || interval <= 0 {
		return time.Second
	}
	return interval
}
