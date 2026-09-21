package contracts

import (
	"errors"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/jeelan-ds786/music-recommender-system/music-event-streaming-service/internal/contracts/identitypb"
	"github.com/jeelan-ds786/music-recommender-system/music-event-streaming-service/internal/contracts/playbackpb"
	"github.com/jeelan-ds786/music-recommender-system/music-event-streaming-service/internal/topics"
)

func TestDecoderExtractsExistingProducerEventIDs(t *testing.T) {
	tests := []struct {
		name    string
		topic   string
		message proto.Message
	}{
		{name: "playback", topic: topics.PlaybackEventV1, message: &playbackpb.PlaybackEventV1{EventId: "playback-id", SchemaVersion: 1}},
		{name: "registered", topic: topics.UserRegistered, message: &identitypb.UserRegistered{Metadata: &identitypb.EventMetadata{EventId: "registered-id", SchemaVersion: 1}}},
		{name: "preference", topic: topics.UserPreferenceUpdated, message: &identitypb.UserPreferenceUpdated{Metadata: &identitypb.EventMetadata{EventId: "preference-id", SchemaVersion: 1}}},
		{name: "playlist", topic: topics.UserPlaylistUpdated, message: &identitypb.PlaylistUpdated{Metadata: &identitypb.EventMetadata{EventId: "playlist-id", SchemaVersion: 1}}},
	}

	decoder := Decoder{}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			payload, err := proto.Marshal(test.message)
			if err != nil {
				t.Fatal(err)
			}
			eventID, err := decoder.EventID(test.topic, payload)
			if err != nil {
				t.Fatal(err)
			}
			if eventID == "" {
				t.Fatal("event ID is empty")
			}
		})
	}
}

func TestDecoderRejectsUnknownSchemaVersion(t *testing.T) {
	payload, err := proto.Marshal(&playbackpb.PlaybackEventV1{EventId: "event-id", SchemaVersion: 2})
	if err != nil {
		t.Fatal(err)
	}
	_, err = (Decoder{}).EventID(topics.PlaybackEventV1, payload)
	if !errors.Is(err, ErrInvalidContract) {
		t.Fatalf("error = %v, want ErrInvalidContract", err)
	}
}
