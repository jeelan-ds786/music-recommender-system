
package main

import (
	"fmt"
	"log"

	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/config"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/db"
)

func main() {
	
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	pgDB, err := db.ConnectPostgres(cfg.DB_URL)
	if err != nil {
		log.Fatalf("Could not connect to Postgres: %v", err)
	}
	defer pgDB.Close()
	fmt.Println("Successfully connected to Postgres!")

	redisClient, err := db.ConnectRedis(cfg.REDIS_URL)
	if err != nil {
		log.Fatalf("Could not connect to Redis: %v", err)
	}
	defer redisClient.Close()
	fmt.Println("Successfully connected to Redis!")

	fmt.Printf("Service is live on port %s\n", cfg.PORT)
}