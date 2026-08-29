package main

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/joho/godotenv"

	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/auth"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/db"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/event"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/httplog"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/logger"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/preference"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/profile"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/refresh"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/reqid"
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

	userRepo := user.NewRepository(pool)
	refreshRepo := refresh.NewRepository(pool)
	profileRepo := profile.NewRepository(pool)
	jwtService := token.NewJWTService(jwtSecret)
	tokenService := token.NewService(jwtService, refreshRepo, profileRepo, appLogger)
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

		pub = kafkaPub
	} else {
		pub = event.NoopPublisher{}
		appLogger.Info("", "KAFKA_BROKERS not set, using NoopPublisher")
	}

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

	preferenceRepo := preference.NewRepository(pool)
	profileService := profile.NewProfileService(profileRepo, preferenceRepo, userRepo, appLogger)
	profileHandler := profile.NewHandler(profileService, appLogger)

	preferenceService := preference.NewService(preferenceRepo, appLogger, emitter)
	preferenceHandler := preference.NewHandler(preferenceService, appLogger)

	r := chi.NewRouter()
	r.Use(reqid.Middleware)
	r.Use(httplog.Middleware(appLogger))

	r.Route("/auth", func(r chi.Router) {
		r.Post("/register", authHandler.Register)
		r.Post("/login", authHandler.Login)
		r.Post("/refresh", authHandler.Refresh)
	})

	r.With(auth.AuthMiddleware(jwtService, appLogger)).Get("/me", profileHandler.Me)
	r.With(auth.AuthMiddleware(jwtService, appLogger)).Patch("/me", profileHandler.PatchMe)

	r.With(auth.AuthMiddleware(jwtService, appLogger)).Post("/me/onboarding", preferenceHandler.Onboarding)
	r.With(auth.AuthMiddleware(jwtService, appLogger)).Post("/me/likes/songs/{songID}", preferenceHandler.LikeSong)
	r.With(auth.AuthMiddleware(jwtService, appLogger)).Delete("/me/likes/songs/{songID}", preferenceHandler.UnlikeSong)
	r.With(auth.AuthMiddleware(jwtService, appLogger)).Get("/me/likes/songs", preferenceHandler.ListLikedSongs)
	r.With(auth.AuthMiddleware(jwtService, appLogger)).Post("/me/following/artists/{artistID}", preferenceHandler.FollowArtist)
	r.With(auth.AuthMiddleware(jwtService, appLogger)).Delete("/me/following/artists/{artistID}", preferenceHandler.UnfollowArtist)

	log.Printf("server listening on :%s", port)

	if err := http.ListenAndServe(":"+port, r); err != nil {
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
