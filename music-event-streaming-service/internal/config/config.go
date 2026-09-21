package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jeelan-ds786/music-recommender-system/music-event-streaming-service/internal/topics"
)

type Config struct {
	Port         string
	DatabaseURL  string
	Brokers      []string
	Topics       []string
	GroupID      string
	Concurrency  int
	DrainTimeout time.Duration
}

func Load() (Config, error) {
	config := Config{
		Port:         valueOrDefault("PORT", "8083"),
		DatabaseURL:  os.Getenv("DB_URL"),
		Brokers:      split(os.Getenv("KAFKA_BROKERS")),
		Topics:       split(valueOrDefault("KAFKA_TOPICS", topics.PlaybackEventV1)),
		GroupID:      valueOrDefault("KAFKA_GROUP_ID", "music-event-streaming-v1"),
		Concurrency:  intOrDefault("CONSUMER_CONCURRENCY", 4),
		DrainTimeout: durationOrDefault("DRAIN_TIMEOUT", 30*time.Second),
	}
	if config.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DB_URL is required")
	}
	if len(config.Brokers) == 0 {
		return Config{}, fmt.Errorf("KAFKA_BROKERS is required")
	}
	if len(config.Topics) == 0 {
		return Config{}, fmt.Errorf("KAFKA_TOPICS is required")
	}
	if config.Concurrency < 1 || config.Concurrency > 32 {
		return Config{}, fmt.Errorf("CONSUMER_CONCURRENCY must be between 1 and 32")
	}
	return config, nil
}

func valueOrDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func split(value string) []string {
	parts := strings.Split(value, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			values = append(values, value)
		}
	}
	return values
}

func intOrDefault(name string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(name))
	if err != nil {
		return fallback
	}
	return value
}

func durationOrDefault(name string, fallback time.Duration) time.Duration {
	value, err := time.ParseDuration(os.Getenv(name))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
