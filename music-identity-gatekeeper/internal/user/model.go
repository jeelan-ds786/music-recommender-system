package user

import (
	"time"

	"github.com/google/uuid"
)

const (
	TierFree    = "free"
	TierPremium = "premium"
	TierAdmin   = "admin"
)

type User struct {
	ID             uuid.UUID
	Email          string
	HashedPassword string
	AuthProvider   string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
