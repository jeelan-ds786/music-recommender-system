package topics

import "time"

const (
	UserRegistered         = "user.registered"
	UserPreferenceUpdated  = "user.preference.updated"
	UserPlaylistUpdated    = "user.playlist.updated"
	PlaybackEventV1        = "playback.event.v1"
	RecommendationFeedback = "recommendation.feedback.v1"
	Retry                  = "event.retry.v1"
	DeadLetter             = "event.dlq.v1"
	HeaderContentType      = "content-type"
	HeaderEventType        = "event-type"
	HeaderSchemaVersion    = "schema-version"
)

type Definition struct {
	Name       string
	Partitions int
	Retention  time.Duration
	Key        string
	Headers    []string
	Schema     int
	Producer   string
	Consumers  string
}

var Definitions = []Definition{
	{Name: UserRegistered, Partitions: 1, Retention: 7 * 24 * time.Hour, Key: "user_id", Schema: 1, Producer: "music-identity-gatekeeper", Consumers: "streaming platform"},
	{Name: UserPreferenceUpdated, Partitions: 1, Retention: 7 * 24 * time.Hour, Key: "user_id", Schema: 1, Producer: "music-identity-gatekeeper", Consumers: "streaming platform"},
	{Name: UserPlaylistUpdated, Partitions: 1, Retention: 7 * 24 * time.Hour, Key: "user_id", Schema: 1, Producer: "music-identity-gatekeeper", Consumers: "streaming platform"},
	{Name: PlaybackEventV1, Partitions: 3, Retention: 7 * 24 * time.Hour, Key: "user_id", Headers: []string{HeaderContentType, HeaderEventType, HeaderSchemaVersion}, Schema: 1, Producer: "music-playback-service", Consumers: "streaming platform"},
	{Name: RecommendationFeedback, Partitions: 3, Retention: 7 * 24 * time.Hour, Key: "user_id", Headers: []string{HeaderContentType, HeaderEventType, HeaderSchemaVersion}, Schema: 1, Producer: "future recommendation API", Consumers: "streaming platform"},
	{Name: Retry, Partitions: 3, Retention: 7 * 24 * time.Hour, Key: "original key", Headers: []string{HeaderContentType, HeaderSchemaVersion}, Schema: 1, Producer: "music-event-streaming-service", Consumers: "streaming retry workers"},
	{Name: DeadLetter, Partitions: 1, Retention: 30 * 24 * time.Hour, Key: "original key", Headers: []string{HeaderContentType, HeaderSchemaVersion}, Schema: 1, Producer: "music-event-streaming-service", Consumers: "operators and replay tooling"},
}
