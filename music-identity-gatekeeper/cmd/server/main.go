package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"google.golang.org/grpc"

	"github.com/joho/godotenv"

	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/auth"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/db"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/event"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/health"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/httplog"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/logger"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/metrics"
	oauthflow "github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/oauth"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/playlist"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/preference"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/profile"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/profile/profilepb"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/refresh"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/reqid"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/revocation"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/token"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/user"
)

func main() {
	ctx := context.Background()

	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	dsn := os.Getenv("DB_URL")
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET is required")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	appLogger := logger.New(logger.ParseLevel(os.Getenv("LOG_LEVEL")))
	appLogger.Info("", "connecting to postgres at %s", maskDSN(dsn))

	pool, err := db.NewPostgresPool(ctx, dsn, db.NewQueryTracer(appLogger))
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	redisClient, err := db.NewRedisClient(redisAddr)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = redisClient.Close() }()
	blacklist := revocation.NewStore(redisClient)
	serviceMetrics := metrics.New(pool)
	healthChecks := []health.Check{
		{Name: "postgres", Ping: pool.Ping},
		{Name: "redis", Ping: func(ctx context.Context) error { return redisClient.Ping(ctx).Err() }},
	}

	userRepo := user.NewRepository(pool)
	refreshRepo := refresh.NewRepository(pool)
	profileRepo := profile.NewRepository(pool)
	jwtService := token.NewJWTService(jwtSecret)
	tokenService := token.NewService(jwtService, refreshRepo, profileRepo, appLogger, blacklist)
	_ = profile.NewService(profileRepo, tokenService)

	// --- Event infrastructure (E1-SS-09) ---
	// pub is built before emitter: the Emitter now attempts a direct,
	// synchronous publish immediately after enqueueing (not just the
	// Relay), so it needs the same Publisher instance the Relay uses.
	outbox := event.NewOutbox(pool, appLogger)

	var pub event.Publisher
	if brokers := os.Getenv("KAFKA_BROKERS"); brokers != "" {
		brokerList := strings.Split(brokers, ",")
		kafkaPub := event.NewKafkaPublisher(brokerList, appLogger)

		pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		if err := kafkaPub.Ping(pingCtx); err != nil {
			appLogger.Error("", "could not connect to Kafka broker(s) %v: %v (events will still be durably enqueued; the relay fallback will retry once Kafka is reachable)", brokerList, err)
		} else {
			appLogger.Info("", "connected to Kafka broker(s): %v", brokerList)
		}
		cancel()
		healthChecks = append(healthChecks, health.Check{Name: "kafka", Ping: kafkaPub.Ping})

		pub = kafkaPub
	} else {
		pub = event.NoopPublisher{}
		appLogger.Info("", "KAFKA_BROKERS not set, using NoopPublisher")
	}
	pub = serviceMetrics.InstrumentPublisher(pub)

	emitter := event.NewEmitter(outbox, appLogger, pub)

	// KAFKA_RELAY_INTERVAL controls how often the relay's fallback sweep
	// wakes up to retry anything still 'pending' (i.e. events whose direct
	// publish attempt above failed) — e.g. "1m", "2m", "5m" — Go duration
	// syntax. Was previously hardcoded to 1 second (constant DB polling);
	// default here is deliberately much coarser since this is purely a
	// catch-up mechanism now, not the primary publish path.
	relayInterval := parseDurationEnv(appLogger, "KAFKA_RELAY_INTERVAL", time.Minute)

	relay := event.NewRelay(outbox, pub, appLogger, event.WithPollInterval(relayInterval))

	// KAFKA_RELAY_ENABLED only gates this fallback sweep — it never
	// affects the direct publish attempt in Emitter above, which always
	// runs. Defaults to enabled so existing local setups keep working
	// unchanged.
	if parseBoolEnv(appLogger, "KAFKA_RELAY_ENABLED", true) {
		relay.Start()
		appLogger.Info("", "Kafka relay fallback scheduler started, interval=%s (watching kafka_integration for pending events)", relayInterval)
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = relay.Shutdown(shutdownCtx)
		}()
	} else {
		appLogger.Info("", "KAFKA_RELAY_ENABLED is false, relay fallback scheduler not started (direct publish attempts on emit still occur)")
	}

	authService := auth.NewService(userRepo, tokenService, appLogger, emitter)

	authHandler := auth.NewHandler(authService)

	googleClientID := os.Getenv("GOOGLE_CLIENT_ID")
	googleClientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	googleRedirectURL := os.Getenv("GOOGLE_REDIRECT_URL")
	if googleClientID == "" || googleClientSecret == "" || googleRedirectURL == "" {
		log.Fatal("GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET, and GOOGLE_REDIRECT_URL are required")
	}
	oauthStates := oauthflow.NewStateManager(oauthflow.NewRedisStateStore(redisClient))
	googleProvider := oauthflow.NewGoogleProvider(googleClientID, googleClientSecret, googleRedirectURL)
	oauthAccounts := oauthflow.NewAccountRepository(pool)
	oauthService := oauthflow.NewService(oauthStates, googleProvider, userRepo, oauthAccounts, profileRepo, tokenService)
	oauthHandler := oauthflow.NewHandler(oauthService)

	preferenceRepo := preference.NewRepository(pool)
	profileService := profile.NewProfileService(profileRepo, preferenceRepo, userRepo, appLogger)
	profileHandler := profile.NewHandler(profileService, appLogger)

	preferenceService := preference.NewService(preferenceRepo, appLogger, emitter)
	preferenceHandler := preference.NewHandler(preferenceService, appLogger)

	playlistRepo := playlist.NewRepository(pool)
	playlistService := playlist.NewService(playlistRepo, appLogger)
	playlistHandler := playlist.NewHandler(playlistService, appLogger)

	r := chi.NewRouter()
	r.Use(reqid.Middleware)
	r.Use(httplog.Middleware(appLogger, serviceMetrics))
	authenticate := auth.AuthMiddleware(jwtService, appLogger, blacklist)
	healthHandler := health.NewHandler(2*time.Second, healthChecks...)

	r.Handle("/metrics", serviceMetrics.Handler())
	r.Get("/health/live", healthHandler.Live)
	r.Get("/health/ready", healthHandler.Ready)

	r.Route("/auth", func(r chi.Router) {
		r.Post("/register", authHandler.Register)
		r.Post("/login", authHandler.Login)
		r.Post("/refresh", authHandler.Refresh)
		r.Get("/google", oauthHandler.Begin)
		r.Get("/google/callback", oauthHandler.Callback)
		r.With(authenticate).Post("/logout", authHandler.Logout)
	})

	r.With(authenticate).Get("/me", profileHandler.Me)
	r.With(authenticate).Patch("/me", profileHandler.PatchMe)

	r.With(authenticate).Post("/me/onboarding", preferenceHandler.Onboarding)
	r.With(authenticate).Post("/me/likes/songs/{songID}", preferenceHandler.LikeSong)
	r.With(authenticate).Delete("/me/likes/songs/{songID}", preferenceHandler.UnlikeSong)
	r.With(authenticate).Get("/me/likes/songs", preferenceHandler.ListLikedSongs)
	r.With(authenticate).Post("/me/following/artists/{artistID}", preferenceHandler.FollowArtist)
	r.With(authenticate).Delete("/me/following/artists/{artistID}", preferenceHandler.UnfollowArtist)

	r.With(authenticate).Post("/me/playlists", playlistHandler.Create)
	r.With(authenticate).Get("/me/playlists", playlistHandler.List)
	r.With(authenticate).Get("/me/playlists/{playlistID}", playlistHandler.Get)
	r.With(authenticate).Patch("/me/playlists/{playlistID}", playlistHandler.Patch)
	r.With(authenticate).Delete("/me/playlists/{playlistID}", playlistHandler.Delete)
	r.With(authenticate).Post("/me/playlists/{playlistID}/songs/{songID}", playlistHandler.AddSong)
	r.With(authenticate).Delete("/me/playlists/{playlistID}/songs/{songID}", playlistHandler.RemoveSong)

	httpListener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatal(err)
	}
	grpcListener, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatal(err)
	}

	httpServer := &http.Server{
		Addr:              ":" + port,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}
	grpcServer := grpc.NewServer()
	profilepb.RegisterIdentityServiceServer(grpcServer, profile.NewGRPCServer(profileService))

	shutdownContext, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Printf("HTTP server listening on :%s", port)
	log.Printf("internal gRPC server listening on :50051")
	if err := serve(shutdownContext, httpServer, httpListener, grpcServer, grpcListener); err != nil {
		log.Fatal(err)
	}
}

// parseBoolEnv parses an on/off env var as a strict boolean via
// strconv.ParseBool (accepts "1", "t", "T", "TRUE", "true", "True", "0",
// "f", "F", "FALSE", "false", "False"). Unset falls back to def silently;
// set but unparseable falls back to def with a logged error, rather than
// guessing at the caller's intent.
func parseBoolEnv(log *logger.Logger, name string, def bool) bool {
	raw := os.Getenv(name)
	if raw == "" {
		return def
	}

	parsed, err := strconv.ParseBool(raw)
	if err != nil {
		log.Error("", "invalid %s=%q (expected a boolean: true/false, 1/0, t/f), falling back to %t: %v", name, raw, def, err)
		return def
	}

	return parsed
}

// parseDurationEnv parses a Go duration string (e.g. "1m", "2m", "30s")
// via time.ParseDuration. Unset falls back to def silently; unparseable or
// non-positive falls back to def with a logged error — a zero or negative
// interval would turn the scheduler back into a tight loop, defeating its
// purpose.
func parseDurationEnv(log *logger.Logger, name string, def time.Duration) time.Duration {
	raw := os.Getenv(name)
	if raw == "" {
		return def
	}

	parsed, err := time.ParseDuration(raw)
	if err != nil {
		log.Error("", "invalid %s=%q (expected a Go duration, e.g. 1m, 2m, 5m, 30s), falling back to %s: %v", name, raw, def, err)
		return def
	}
	if parsed <= 0 {
		log.Error("", "invalid %s=%q (must be positive), falling back to %s", name, raw, def)
		return def
	}

	return parsed
}

func maskDSN(dsn string) string {
	u, err := url.Parse(dsn)
	if err != nil {
		return "invalid DB_URL"
	}

	if u.User != nil {
		u.User = url.UserPassword(u.User.Username(), "****")
	}

	return u.String()
}
