package playlist

import (
	"time"

	"github.com/google/uuid"
)

type Playlist struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	Name        string
	Description *string
	IsPublic    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type PlaylistSong struct {
	SongID   uuid.UUID
	Position int
	AddedAt  time.Time
}
