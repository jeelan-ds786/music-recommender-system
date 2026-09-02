package logger

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"
)

func TestInfoFieldsWritesStructuredJSON(t *testing.T) {
	var output bytes.Buffer
	log := NewWithWriter(LevelInfo, &output)

	log.InfoFields("request-123", "request completed",
		String("method", "GET"),
		Int("status", 204),
		Duration("latency", 25*time.Millisecond),
	)

	var entry map[string]any
	if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
		t.Fatalf("log output is not JSON: %v", err)
	}

	for key, want := range map[string]any{
		"level":      "info",
		"message":    "request completed",
		"request_id": "request-123",
		"method":     "GET",
		"status":     float64(204),
	} {
		if got := entry[key]; got != want {
			t.Errorf("%s = %v, want %v", key, got, want)
		}
	}

	if _, ok := entry["latency"]; !ok {
		t.Error("latency field is missing")
	}
}
