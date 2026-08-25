package profile

import (
	"time"

	"github.com/google/uuid"
)

type Profile struct {
	UserID           uuid.UUID
	DisplayName      *string
	AvatarURL        *string
	Country          *string
	Language         *string
	BirthYear        *int16
	SubscriptionTier string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}
