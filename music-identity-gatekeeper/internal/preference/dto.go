package preference

import "github.com/google/uuid"

// PreferenceResponse is the transport-agnostic shape returned to both
// the HTTP and gRPC layers. It carries no json/protobuf tags of its own;
// each transport maps it into its own wire format.
type PreferenceResponse struct {
	LikedSongIDs      []uuid.UUID
	FollowedArtistIDs []uuid.UUID
	GenreSeeds        []string
	LanguagePrefs     []string
}
