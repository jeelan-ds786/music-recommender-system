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

// SessionSummaryResponse is derived from the whole session's events at
// query time — see SessionOverview in model.go.
type SessionSummaryResponse struct {
	SessionID  uuid.UUID `json:"session_id"`
	EventCount int       `json:"event_count"`
	StartedAt  time.Time `json:"started_at"`
	EndedAt    time.Time `json:"ended_at"`
	DurationMS int64     `json:"duration_ms"`
}

// EventQuality flags are computed fresh on every read, never stored.
// CompletionPercentage is nil when duration_ms is unknown (nil or 0).
type EventQuality struct {
	Late                 bool     `json:"late"`
	OutOfOrder           bool     `json:"out_of_order"`
	CompletionPercentage *float64 `json:"completion_percentage"`
}

type SessionEventResponse struct {
	EventID       uuid.UUID    `json:"event_id"`
	ClientEventID uuid.UUID    `json:"client_event_id"`
	SongID        uuid.UUID    `json:"song_id"`
	EventType     EventType    `json:"event_type"`
	OccurredAt    time.Time    `json:"occurred_at"`
	IngestedAt    time.Time    `json:"ingested_at"`
	PositionMS    int64        `json:"position_ms"`
	DurationMS    *int64       `json:"duration_ms"`
	DeviceType    string       `json:"device_type"`
	Quality       EventQuality `json:"quality"`
}

type SessionEventsPage struct {
	Session SessionSummaryResponse `json:"session"`
	Events  []SessionEventResponse `json:"events"`
	// Not omitempty — the last page returns an explicit null, matching
	// preference.LikedSongsPage's convention.
	NextCursor *string `json:"next_cursor"`
}
