package event

import (
	"strconv"
	"time"

	"github.com/google/uuid"
)

const (
	PlaybackTopic       = "playback.event.v1"
	SchemaVersion       = 1
	Producer            = "music-playback-service"
	HeaderContentType   = "content-type"
	HeaderEventType     = "event-type"
	HeaderSchemaVersion = "schema-version"
	ProtobufContentType = "application/x-protobuf"
)

type Message struct {
	ID              uuid.UUID
	PlaybackEventID uuid.UUID
	Topic           string
	Key             string
	EventType       string
	SchemaVersion   int
	Headers         map[string]string
	Payload         []byte
	OccurredAt      time.Time
	CreatedAt       time.Time
	PublishedAt     *time.Time
	Attempts        int
	LastError       string
}

func PlaybackHeaders(eventType string) map[string]string {
	return map[string]string{
		HeaderContentType:   ProtobufContentType,
		HeaderEventType:     eventType,
		HeaderSchemaVersion: strconv.Itoa(SchemaVersion),
	}
}
