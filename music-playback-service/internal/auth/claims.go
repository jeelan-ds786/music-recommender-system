package auth

import "github.com/golang-jwt/jwt/v5"

// Claims mirrors music-identity-gatekeeper's internal/token.Claims exactly
// (same JSON field names, same HS256 signing). This service only verifies
// access tokens issued by the identity service — it never issues or
// refreshes tokens itself, so there's no need for the rest of that
// service's token package (refresh rotation, blacklist, etc.).
type Claims struct {
	UserID string `json:"user_id"`
	Tier   string `json:"tier"`

	jwt.RegisteredClaims
}
