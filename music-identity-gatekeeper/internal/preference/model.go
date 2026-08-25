package preference

import (
	"time"

	"github.com/google/uuid"
)

type Preference struct {
	UserID            uuid.UUID
	LikedSongIDs      []uuid.UUID
	FollowedArtistIDs []uuid.UUID
	GenreSeeds        []string
	LanguagePrefs     []string
	UpdatedAt         time.Time
}

type LikedSong struct {
	SongID    uuid.UUID
	CreatedAt time.Time
}

type FollowedArtist struct {
	ArtistID  uuid.UUID
	CreatedAt time.Time
}
