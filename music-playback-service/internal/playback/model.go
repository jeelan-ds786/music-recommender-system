package playback

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type EventType string

const (
	EventTypePlay     EventType = "play"
	EventTypePause    EventType = "pause"
	EventTypeSeek     EventType = "seek"
	EventTypeSkip     EventType = "skip"
	EventTypeReplay   EventType = "replay"
	EventTypeComplete EventType = "complete"
)

type Event struct {
	ID            uuid.UUID
	ClientEventID uuid.UUID
	UserID        uuid.UUID
	SongID        uuid.UUID
	SessionID     uuid.UUID
	Type          EventType
	OccurredAt    time.Time
	PositionMS    int64
	DurationMS    *int64
	DeviceType    string
	Context       json.RawMessage
	IngestedAt    time.Time
}

// SessionOverview is derived at query time from a session's full set of
// events — never persisted, never a state machine. See
// repository.go's GetSessionEvents/detectOutOfOrder for how OutOfOrder is
// computed.
type SessionOverview struct {
	EventCount int
	StartedAt  time.Time
	EndedAt    time.Time
	DurationMS int64
	OutOfOrder map[uuid.UUID]bool
}
