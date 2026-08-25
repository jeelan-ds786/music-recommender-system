package token

import (
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"time"
)

type Claims struct {
	UserID string `json:"user_id"`
	Tier   string `json:"tier"`

	jwt.RegisteredClaims
}

// NewAccessTokenClaims creates the claims that will be placed
// inside a new access token.
//
// This function does NOT create/sign the JWT.
//
// It only creates the PAYLOAD.
//
// Later jwt.go will take these claims and sign them.
func NewAccessTokenClaims(
	userID uuid.UUID,
	tier string,
	ttl time.Duration,
) Claims {

	// Capture the current time once.
	//
	// We use the same timestamp for both:
	//
	// iat → issued-at time
	// exp → expiration time
	//
	// Using one value also prevents tiny timing differences
	// between the two timestamps.
	now := time.Now()

	// Return the complete JWT claims object.
	return Claims{

		// Convert UUID into its standard string representation.
		//
		// Example:
		//
		// uuid.UUID
		//      ↓
		// "550e8400-e29b-41d4-a716-446655440000"
		UserID: userID.String(),

		// Store the subscription tier used for authorization.
		Tier: tier,

		// Standard JWT claims.
		RegisteredClaims: jwt.RegisteredClaims{

			// IssuedAt records when the access token was created.
			//
			// Example:
			//
			// "iat": 1755500000
			IssuedAt: jwt.NewNumericDate(now),

			// ExpiresAt determines when the token becomes invalid.
			//
			// ttl will eventually be:
			//
			// 15 * time.Minute
			//
			// Therefore:
			//
			// expiration = now + 15 minutes
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
}
