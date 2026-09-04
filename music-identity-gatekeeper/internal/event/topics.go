package event

const (
	TopicUserRegistered        = "user.registered"
	TopicUserPreferenceUpdated = "user.preference.updated"
	TopicUserPlaylistUpdated   = "user.playlist.updated"

	PlaylistOperationCreated     = "created"
	PlaylistOperationUpdated     = "updated"
	PlaylistOperationDeleted     = "deleted"
	PlaylistOperationSongAdded   = "song_added"
	PlaylistOperationSongRemoved = "song_removed"
)
