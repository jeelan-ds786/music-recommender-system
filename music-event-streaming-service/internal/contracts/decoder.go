package contracts

import (
	"errors"
	"fmt"

	"google.golang.org/protobuf/proto"

	"github.com/jeelan-ds786/music-recommender-system/music-event-streaming-service/internal/contracts/identitypb"
	"github.com/jeelan-ds786/music-recommender-system/music-event-streaming-service/internal/contracts/playbackpb"
	"github.com/jeelan-ds786/music-recommender-system/music-event-streaming-service/internal/contracts/recommendationpb"
	"github.com/jeelan-ds786/music-recommender-system/music-event-streaming-service/internal/topics"
)

var (
	ErrUnsupportedTopic = errors.New("unsupported topic")
	ErrInvalidContract  = errors.New("invalid event contract")
)

type Decoder struct{}

func (Decoder) EventID(topic string, payload []byte) (string, error) {
	var eventID string
	var schemaVersion int32

	switch topic {
	case topics.PlaybackEventV1:
		var event playbackpb.PlaybackEventV1
		if err := proto.Unmarshal(payload, &event); err != nil {
			return "", fmt.Errorf("%w: %v", ErrInvalidContract, err)
		}
		eventID, schemaVersion = event.EventId, event.SchemaVersion
	case topics.UserRegistered:
		var event identitypb.UserRegistered
		if err := proto.Unmarshal(payload, &event); err != nil {
			return "", fmt.Errorf("%w: %v", ErrInvalidContract, err)
		}
		eventID, schemaVersion = metadata(event.Metadata)
	case topics.UserPreferenceUpdated:
		var event identitypb.UserPreferenceUpdated
		if err := proto.Unmarshal(payload, &event); err != nil {
			return "", fmt.Errorf("%w: %v", ErrInvalidContract, err)
		}
		eventID, schemaVersion = metadata(event.Metadata)
	case topics.UserPlaylistUpdated:
		var event identitypb.PlaylistUpdated
		if err := proto.Unmarshal(payload, &event); err != nil {
			return "", fmt.Errorf("%w: %v", ErrInvalidContract, err)
		}
		eventID, schemaVersion = metadata(event.Metadata)
	case topics.RecommendationFeedback:
		var event recommendationpb.RecommendationFeedbackV1
		if err := proto.Unmarshal(payload, &event); err != nil {
			return "", fmt.Errorf("%w: %v", ErrInvalidContract, err)
		}
		eventID, schemaVersion = event.EventId, event.SchemaVersion
	default:
		return "", fmt.Errorf("%w: %s", ErrUnsupportedTopic, topic)
	}

	if eventID == "" || schemaVersion != 1 {
		return "", ErrInvalidContract
	}
	return eventID, nil
}

func metadata(value *identitypb.EventMetadata) (string, int32) {
	if value == nil {
		return "", 0
	}
	return value.EventId, value.SchemaVersion
}
