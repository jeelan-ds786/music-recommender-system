package refresh

import (
	"time"

	"github.com/google/uuid"
)

// RefreshToken represents one refresh token stored in the database.
//
// Important:
// We NEVER store the actual/raw refresh token in the database.
// We only store its SHA-256 hash.
//
// Example:
//
// Raw token:
//
//	"a8f91c...."
//
// Database:
//
//	SHA256("a8f91c....") → "7d4e..."
//
// If the database is compromised, the attacker cannot directly
// use the stored hash as the refresh token.
type RefreshToken struct {
	TokenHash string
	UserID    uuid.UUID
	ExpiresAt time.Time
	Revoked   bool
	CreatedAt time.Time
}
