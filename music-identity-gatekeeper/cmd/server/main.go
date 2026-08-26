package main

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/go-chi/chi/v5"

	"github.com/joho/godotenv"

	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/auth"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/db"
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

	authService := auth.NewService(userRepo, tokenService, appLogger)

	authHandler := auth.NewHandler(authService)

	preferenceRepo := preference.NewRepository(pool)
	profileService := profile.NewProfileService(profileRepo, preferenceRepo, userRepo, appLogger)
	profileHandler := profile.NewHandler(profileService, appLogger)

	preferenceService := preference.NewService(preferenceRepo, appLogger)
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
