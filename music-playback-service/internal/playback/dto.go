package playback

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// IngestRequest is the wire shape of one telemetry event. UUID and
// time.Time fields let encoding/json reject malformed values during
// decode (handler.go treats any decode failure as a generic
// INVALID_REQUEST_BODY, matching internal/playlist's convention in
// music-identity-gatekeeper) — user_id is deliberately absent here; it
// only ever comes from the authenticated request context.
type IngestRequest struct {
	ClientEventID uuid.UUID       `json:"client_event_id"`
	SongID        uuid.UUID       `json:"song_id"`
	SessionID     uuid.UUID       `json:"session_id"`
	EventType     EventType       `json:"event_type"`
	OccurredAt    time.Time       `json:"occurred_at"`
	PositionMS    int64           `json:"position_ms"`
	DurationMS    *int64          `json:"duration_ms,omitempty"`
	DeviceType    string          `json:"device_type"`
	Context       json.RawMessage `json:"context,omitempty"`
}

type BatchIngestRequest struct {
	Events []IngestRequest `json:"events"`
}

// IngestStatus is "accepted" for a newly stored event and "duplicate" for
// one that already existed under the same (user_id, client_event_id) —
// both are successes, never an error.
type IngestStatus string

const (
	IngestStatusAccepted  IngestStatus = "accepted"
	IngestStatusDuplicate IngestStatus = "duplicate"
)

type IngestResponse struct {
	EventID       uuid.UUID    `json:"event_id"`
	ClientEventID uuid.UUID    `json:"client_event_id"`
	Status        IngestStatus `json:"status"`
}

type BatchIngestResponse struct {
	Results []IngestResponse `json:"results"`
}
