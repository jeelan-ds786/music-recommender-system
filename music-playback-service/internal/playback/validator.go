package playback

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	// MaxBatchSize bounds POST /v1/playback/events:batch — matches the
	// cap fixed in Sprint 3's architecture decisions.
	MaxBatchSize = 100

	// MaxContextBytes mirrors the DB-level CHECK on playback_events.context
	// (octet_length(context::text) <= 4096) — validated here too so an
	// oversized context fails fast with a structured 400 instead of a raw
	// DB constraint error.
	MaxContextBytes = 4096

	// MaxFutureSkew rejects telemetry claiming to occur further in the
	// future than a client clock could plausibly justify.
	MaxFutureSkew = 24 * time.Hour
)

var validEventTypes = map[EventType]struct{}{
	EventTypePlay:     {},
	EventTypePause:    {},
	EventTypeSeek:     {},
	EventTypeSkip:     {},
	EventTypeReplay:   {},
	EventTypeComplete: {},
}

type ValidationError struct {
	Error string
	Field string
}

// ValidateIngest checks one event's semantic rules (the ones encoding/json
// can't enforce by rejecting decode). fieldPrefix is "" for the single
// endpoint and "events[<i>]." for a batch item, so error payloads point at
// exactly which event failed.
func ValidateIngest(req IngestRequest, now time.Time, fieldPrefix string) *ValidationError {
	field := func(name string) string { return fieldPrefix + name }

	if req.ClientEventID == uuid.Nil {
		return &ValidationError{Error: "CLIENT_EVENT_ID_REQUIRED", Field: field("client_event_id")}
	}
	if req.SongID == uuid.Nil {
		return &ValidationError{Error: "SONG_ID_REQUIRED", Field: field("song_id")}
	}
	if req.SessionID == uuid.Nil {
		return &ValidationError{Error: "SESSION_ID_REQUIRED", Field: field("session_id")}
	}
	if _, ok := validEventTypes[req.EventType]; !ok {
		return &ValidationError{Error: "INVALID_EVENT_TYPE", Field: field("event_type")}
	}
	if req.OccurredAt.IsZero() {
		return &ValidationError{Error: "OCCURRED_AT_REQUIRED", Field: field("occurred_at")}
	}
	if req.OccurredAt.After(now.Add(MaxFutureSkew)) {
		return &ValidationError{Error: "OCCURRED_AT_TOO_FAR_IN_FUTURE", Field: field("occurred_at")}
	}
	if req.PositionMS < 0 {
		return &ValidationError{Error: "POSITION_MS_NEGATIVE", Field: field("position_ms")}
	}
	if req.DurationMS != nil && *req.DurationMS < 0 {
		return &ValidationError{Error: "DURATION_MS_NEGATIVE", Field: field("duration_ms")}
	}
	if strings.TrimSpace(req.DeviceType) == "" {
		return &ValidationError{Error: "DEVICE_TYPE_REQUIRED", Field: field("device_type")}
	}
	if len(req.Context) > MaxContextBytes {
		return &ValidationError{Error: "CONTEXT_TOO_LARGE", Field: field("context")}
	}

	return nil
}

// ValidateBatch enforces the batch-level rules (size cap, non-empty) and
// then every event's own rules. Batches are all-or-nothing on malformed
// input: the first failure aborts validation before anything is written.
func ValidateBatch(req BatchIngestRequest, now time.Time) *ValidationError {
	if len(req.Events) == 0 {
		return &ValidationError{Error: "BATCH_EMPTY", Field: "events"}
	}
	if len(req.Events) > MaxBatchSize {
		return &ValidationError{Error: "BATCH_TOO_LARGE", Field: "events"}
	}

	for i, event := range req.Events {
		if validationErr := ValidateIngest(event, now, fmt.Sprintf("events[%d].", i)); validationErr != nil {
			return validationErr
		}
	}

	return nil
}
