package preference

import (
	"time"

	"github.com/google/uuid"
)

// PreferenceResponse is the transport-agnostic shape returned to both
// the HTTP and gRPC layers. It carries no json/protobuf tags of its own;
// each transport maps it into its own wire format.
type PreferenceResponse struct {
	LikedSongIDs      []uuid.UUID
	FollowedArtistIDs []uuid.UUID
	GenreSeeds        []string
	LanguagePrefs     []string
}

// OnboardingRequest is the POST /me/onboarding body. GenreSeeds is capped
// at five per the ticket; LanguagePrefs and FollowedArtistIDs are
// uncapped/optional.
type OnboardingRequest struct {
	GenreSeeds        []string    `json:"genre_seeds" validate:"required,max=5,dive,required"`
	LanguagePrefs     []string    `json:"language_prefs"`
	FollowedArtistIDs []uuid.UUID `json:"followed_artist_ids"`
}

type LikedSongItem struct {
	SongID  uuid.UUID `json:"song_id"`
	LikedAt time.Time `json:"liked_at"`
}

type LikedSongsPage struct {
	Items      []LikedSongItem `json:"items"`
	NextCursor *string         `json:"next_cursor"`
}
