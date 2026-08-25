package profile

import (
	"time"

	"github.com/google/uuid"
)

// PatchMeRequest omits a field via a nil pointer so it's left unchanged.
// An empty string ("") is a present value, not an omission, and clears the
// field — that's intentional, not a bug: encoding/json can't distinguish
// an omitted key from an explicit JSON null on a *string, so nil already
// has to mean "don't touch" either way.
type PatchMeRequest struct {
	DisplayName *string `json:"display_name" validate:"omitempty,max=100"`
	AvatarURL   *string `json:"avatar_url"   validate:"omitempty,url,max=2048"`
	Country     *string `json:"country"      validate:"omitempty,len=2,iso3166_1_alpha2"`
	Language    *string `json:"language"     validate:"omitempty,max=10"`
	BirthYear   *int16  `json:"birth_year"`
}

type AccountFields struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type ProfileFields struct {
	DisplayName *string   `json:"display_name"`
	AvatarURL   *string   `json:"avatar_url"`
	Country     *string   `json:"country"`
	Language    *string   `json:"language"`
	BirthYear   *int16    `json:"birth_year"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type PreferenceFields struct {
	LikedSongIDs      []uuid.UUID `json:"liked_song_ids"`
	FollowedArtistIDs []uuid.UUID `json:"followed_artist_ids"`
	GenreSeeds        []string    `json:"genre_seeds"`
	LanguagePrefs     []string    `json:"language_prefs"`
}

type MeResponse struct {
	Account     AccountFields    `json:"account"`
	Profile     ProfileFields    `json:"profile"`
	Tier        string           `json:"tier"`
	Preferences PreferenceFields `json:"preferences"`
}
