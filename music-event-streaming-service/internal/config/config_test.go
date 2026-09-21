package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("DB_URL", "postgres://example")
	t.Setenv("KAFKA_BROKERS", "kafka:9092")
	t.Setenv("KAFKA_TOPICS", "")
	t.Setenv("CONSUMER_CONCURRENCY", "")

	config, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if config.Port != "8083" || config.GroupID != "music-event-streaming-v1" || config.Concurrency != 4 {
		t.Fatalf("unexpected defaults: %+v", config)
	}
	if len(config.Topics) != 1 || config.Topics[0] != "playback.event.v1" {
		t.Fatalf("topics = %v", config.Topics)
	}
}

func TestLoadRejectsInvalidConcurrency(t *testing.T) {
	t.Setenv("DB_URL", "postgres://example")
	t.Setenv("KAFKA_BROKERS", "kafka:9092")
	t.Setenv("CONSUMER_CONCURRENCY", "33")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want invalid concurrency")
	}
}
