package playlist

import (
	"time"

	"github.com/google/uuid"
)

// CreateRequest is the POST /me/playlists body.
type CreateRequest struct {
	Name        string  `json:"name" validate:"required,max=100"`
	Description *string `json:"description" validate:"omitempty,max=1000"`
	IsPublic    bool    `json:"is_public"`
}

// PatchRequest omits a field via a nil pointer so it's left unchanged, same
// idiom as profile.PatchMeRequest.
type PatchRequest struct {
	Name        *string `json:"name" validate:"omitempty,max=100"`
	Description *string `json:"description" validate:"omitempty,max=1000"`
	IsPublic    *bool   `json:"is_public"`
}

type PlaylistResponse struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description"`
	IsPublic    bool      `json:"is_public"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type PlaylistSongItem struct {
	SongID   uuid.UUID `json:"song_id"`
	Position int       `json:"position"`
	AddedAt  time.Time `json:"added_at"`
}

type PlaylistDetailResponse struct {
	PlaylistResponse
	Songs []PlaylistSongItem `json:"songs"`
}
