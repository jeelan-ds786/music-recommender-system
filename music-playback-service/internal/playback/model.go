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

type Session struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	StartedAt time.Time
	EndedAt   *time.Time
}
