package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"

	"github.com/joho/godotenv"

  	"github.com/jeelan-ds786/music-recommender-system/services/music-identity-gatekeeper/internal/auth"
    "github.com/jeelan-ds786/music-recommender-system/services/music-identity-gatekeeper/internal/db"
    "github.com/jeelan-ds786/music-recommender-system/services/music-identity-gatekeeper/internal/user"
)

func main() {
	ctx := context.Background()

	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	dsn := os.Getenv("DB_URL")

	pool, err := db.NewPostgresPool(ctx, dsn)
	if err != nil {
		log.Fatal(err)
	}

	userRepo := user.NewRepository(pool)

	authService := auth.NewService(userRepo)

	authHandler := auth.NewHandler(authService)

	r := chi.NewRouter()

	r.Route("/auth", func(r chi.Router) {
		r.Post("/register", authHandler.Register)
		r.Post("/login", authHandler.Login)
	})

	log.Println("server listening on :8080")

	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatal(err)
	}
}