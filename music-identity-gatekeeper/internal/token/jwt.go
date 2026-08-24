package token

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// AccessTokenTTL defines how long an access token is valid.
//
// We intentionally keep access tokens short-lived.
// If an access token is stolen, the attacker can only use it
// until this expiration time.
//
// Our architecture:
// Access Token → 15 minutes
// Refresh Token → longer lifetime
const AccessTokenTTL = 15 * time.Minute

// JWTService is responsible for creating and parsing
// JSON Web Tokens.
//
// It owns the secret used to:
//  1. Sign access tokens
//  2. Verify access-token signatures
type JWTService struct {

	// secret is the private signing key used by HS256.
	//
	// IMPORTANT:
	// This secret must NEVER be sent to the client.
	//
	// With HS256 the same secret is used for:
	//
	// Generate:
	//   claims + secret → signature
	//
	// Validate:
	//   claims + secret → verify signature
	secret []byte
}

// NewJWTService creates a new JWT service.
//
// The secret should normally come from an environment variable:
//
//	JWT_SECRET=some-long-random-secret
//
// We convert the string into []byte because the JWT library
// expects the signing key as a byte slice.
func NewJWTService(secret string) *JWTService {

	// Create and return the JWT service.
	return &JWTService{
		secret: []byte(secret),
	}
}

// GenerateAccessToken creates a signed JWT for a user.
//
// Input:
//
//	userID
//	tier
//
// Output:
//
//	signed JWT string
//
// Flow:
//
//	User
//	  ↓
//	Claims
//	  ↓
//	HS256
//	  ↓
//	Signed JWT
func (s *JWTService) GenerateAccessToken(
	userID string,
	tier string,
) (string, error) {

	// Capture the current time.
	//
	// This timestamp becomes the "iat" claim.
	now := time.Now()

	// Create our JWT claims.
	//
	// These become the PAYLOAD section of the JWT.
	//
	// The payload will contain approximately:
	//
	// {
	//   "user_id": "...",
	//   "tier": "free",
	//   "iat": "...",
	//   "exp": "..."
	// }
	claims := Claims{
		// Store the authenticated user's ID.
		UserID: userID,

		// Store how the user authenticated.
		//
		// Example:
		//
		// "local"
		// "google"
		// "github"
		Tier: tier,

		// RegisteredClaims contains standard JWT claims.
		RegisteredClaims: jwt.RegisteredClaims{

			// "iat" = Issued At.
			//
			// This records when the token was generated.
			IssuedAt: jwt.NewNumericDate(now),

			// "exp" = Expiration Time.
			//
			// The token becomes invalid after:
			//
			// current time + 15 minutes
			ExpiresAt: jwt.NewNumericDate(
				now.Add(AccessTokenTTL),
			),
		},
	}

	// Create a new JWT using our claims.
	//
	// HS256 means:
	//
	// HMAC + SHA-256
	//
	// The JWT will have:
	//
	// HEADER.PAYLOAD.SIGNATURE
	token := jwt.NewWithClaims(
		jwt.SigningMethodHS256,
		claims,
	)

	// Sign the JWT using our secret.
	//
	// This produces the final JWT string that we can
	// return to the client.
	return token.SignedString(s.secret)
}

// ParseAccessToken parses and validates an access token.
//
// Flow:
//
//	Raw JWT
//	   ↓
//	Parse
//	   ↓
//	Check algorithm
//	   ↓
//	Verify signature
//	   ↓
//	Verify registered claims such as exp
//	   ↓
//	Return Claims
func (s *JWTService) ParseAccessToken(
	tokenString string,
) (*Claims, error) {

	// ParseWithClaims decodes the JWT and validates it.
	//
	// &Claims{} tells the JWT library what type of payload
	// it should decode the JWT into.
	token, err := jwt.ParseWithClaims(
		tokenString,

		// Destination for the decoded JWT payload.
		&Claims{},

		// keyFunc tells the JWT library which secret to use
		// when verifying the signature.
		func(token *jwt.Token) (any, error) {

			// IMPORTANT SECURITY CHECK.
			//
			// We only allow HS256.
			//
			// If somebody sends a JWT using another signing
			// algorithm, we reject it.
			if token.Method != jwt.SigningMethodHS256 {
				return nil, jwt.ErrTokenSignatureInvalid
			}

			// Return our secret.
			//
			// The JWT library uses this secret to verify
			// that the signature was created by our server.
			return s.secret, nil
		},
	)

	// If parsing or validation failed, reject the token.
	//
	// Possible reasons:
	//
	// - malformed JWT
	// - invalid signature
	// - expired token
	// - invalid claims
	// - unexpected signing algorithm
	if err != nil {
		return nil, err
	}

	// ParseWithClaims returns the claims as a generic
	// jwt.Claims interface.
	//
	// We need to convert it back to our concrete *Claims type.
	claims, ok := token.Claims.(*Claims)

	// Make sure:
	//
	// 1. The claims are actually our Claims type.
	// 2. The JWT passed validation.
	if !ok || !token.Valid {
		return nil, jwt.ErrTokenInvalidClaims
	}

	// Everything passed validation.
	//
	// Return the claims to the caller.
	return claims, nil
}
