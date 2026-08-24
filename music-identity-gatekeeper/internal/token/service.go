package token

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	// We import the refresh package because the token service
	// needs to create and rotate refresh-token database records.
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/refresh"
)

// RefreshTokenTTL defines how long a refresh token remains valid.
//
// Access token:
//
//	15 minutes
//
// Refresh token:
//
//	30 days
//
// The access token is intentionally short-lived.
// The refresh token allows the client to obtain a new
// access token without asking the user to log in again.
//
// IMPORTANT:
// If you already have a refresh-token TTL defined in your
// project/configuration, use that instead of this constant.
const RefreshTokenTTL = 30 * 24 * time.Hour

// TokenPair represents the credentials returned to the client
// after successful authentication.
//
// The client receives:
//
//	{
//	    "access_token": "...",
//	    "refresh_token": "..."
//	}
type TokenPair struct {

	// AccessToken is the short-lived JWT.
	//
	// Lifetime:
	//
	// 15 minutes
	//
	// It is sent with protected API requests:
	//
	// Authorization: Bearer <access_token>
	AccessToken string

	// RefreshToken is the long-lived random token.
	//
	// This is NOT a JWT.
	//
	// It is a cryptographically random opaque token.
	//
	// The raw value is returned ONLY to the client.
	// Its hash is stored in PostgreSQL.
	RefreshToken string
}

// RefreshRepository defines the database operations that
// TokenService needs.
//
// We use an interface instead of directly depending on
// *refresh.Repository.
//
// This gives us:
//
//  1. Loose coupling
//  2. Easier unit testing
//  3. Ability to replace the repository later
//
// The refresh.Repository we created earlier satisfies
// this interface automatically because it implements these
// methods.
type RefreshRepository interface {

	// CreateRefreshToken stores a new refresh-token record.
	CreateRefreshToken(
		ctx context.Context,
		token *refresh.RefreshToken,
	) error

	// GetByHash finds a refresh-token record using its hash.
	GetByHash(
		ctx context.Context,
		tokenHash string,
	) (*refresh.RefreshToken, error)

	// Revoke invalidates an existing refresh token.
	Revoke(
		ctx context.Context,
		tokenHash string,
	) error

	// Rotate atomically revokes the old token and inserts
	// the new token.
	Rotate(
		ctx context.Context,
		oldTokenHash string,
		newToken *refresh.RefreshToken,
	) error
}

type TierProvider interface {
	GetTier(ctx context.Context, userID uuid.UUID) (string, error)
}

// Service is the central authentication token service.
//
// It coordinates:
//
//	Access JWT generation
//	Refresh token generation
//	Refresh token hashing
//	Refresh token persistence
//	Refresh token rotation
type Service struct {

	// jwtService is responsible for access JWTs.
	//
	// It handles:
	//
	// - JWT claims
	// - HS256 signing
	// - JWT parsing
	// - JWT validation
	jwtService *JWTService

	// refreshRepo is responsible for storing and retrieving
	// refresh-token records from PostgreSQL.
	refreshRepo  RefreshRepository
	tierProvider TierProvider
}

// NewService creates the central token service.
//
// Dependencies are injected into the service:
//
//	jwtService
//	refreshRepo
//
// This follows dependency injection rather than creating
// those dependencies internally.
func NewService(
	jwtService *JWTService,
	refreshRepo RefreshRepository,
	tierProvider TierProvider,
) *Service {

	// Return a new token service containing both dependencies.
	return &Service{
		jwtService:   jwtService,
		refreshRepo:  refreshRepo,
		tierProvider: tierProvider,
	}
}

// IssueTokenPair generates both access and refresh tokens.
//
// This is the main method called after successful login.
//
// Flow:
//
//	User
//	  │
//	  ▼
//	IssueTokenPair()
//	  │
//	  ├───────────────┐
//	  ▼               ▼
//	Access JWT     Refresh token
//	  │               │
//	  │             SHA-256
//	  │               │
//	  │               ▼
//	  │          PostgreSQL
//	  │
//	  └───────┬───────┘
//	          ▼
//	      TokenPair
func (s *Service) IssueTokenPair(
	ctx context.Context,
	userID uuid.UUID,
) (*TokenPair, error) {
	tier, err := s.tierProvider.GetTier(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get tier: %w", err)
	}

	// ---------------------------------------------------------
	// STEP 1: Generate the access token
	// ---------------------------------------------------------

	// JWTService handles:
	//
	// - creating claims
	// - setting iat
	// - setting exp
	// - signing using HS256
	accessToken, err := s.jwtService.GenerateAccessToken(
		userID.String(),
		tier,
	)

	// If access-token generation fails, we stop immediately.
	//
	// There is no reason to create a refresh token if
	// the access token could not be generated.
	if err != nil {
		return nil, fmt.Errorf(
			"generate access token: %w",
			err,
		)
	}

	// ---------------------------------------------------------
	// STEP 2: Generate a cryptographically secure refresh token
	// ---------------------------------------------------------

	// GenerateRefreshToken creates the raw opaque token.
	//
	// IMPORTANT:
	// This is NOT a JWT.
	//
	// It is simply a large random value.
	rawRefreshToken, err := generateRefreshToken()

	// If secure random generation fails, we cannot safely
	// create a refresh token.
	if err != nil {
		return nil, fmt.Errorf(
			"generate refresh token: %w",
			err,
		)
	}

	// ---------------------------------------------------------
	// STEP 3: Hash the refresh token
	// ---------------------------------------------------------

	// We NEVER store rawRefreshToken in PostgreSQL.
	//
	// Instead:
	//
	//	raw token
	//	    ↓
	//	SHA-256
	//	    ↓
	//	token hash
	//
	// The client receives the raw token.
	// The database receives only the hash.
	tokenHash := hashRefreshToken(rawRefreshToken)

	// ---------------------------------------------------------
	// STEP 4: Build database model
	// ---------------------------------------------------------

	// Capture the creation time once.
	//
	// We use the same timestamp when calculating expiration
	// and storing created_at.
	now := time.Now()

	// Create the database model.
	refreshRecord := &refresh.RefreshToken{

		// Store ONLY the SHA-256 hash.
		//
		// Never put rawRefreshToken here.
		TokenHash: tokenHash,

		// Associate this refresh token with the authenticated user.
		UserID: userID,

		// Refresh token expires after RefreshTokenTTL.
		ExpiresAt: now.Add(RefreshTokenTTL),

		// Newly issued tokens are not revoked.
		Revoked: false,

		// Record when this token was created.
		CreatedAt: now,
	}

	// ---------------------------------------------------------
	// STEP 5: Store refresh-token hash in PostgreSQL
	// ---------------------------------------------------------

	// Send the database record to our repository.
	//
	// The repository inserts:
	//
	//	token_hash
	//	user_id
	//	expires_at
	//	revoked
	//	created_at
	err = s.refreshRepo.CreateRefreshToken(
		ctx,
		refreshRecord,
	)

	// If database insertion fails, do not return tokens.
	//
	// Otherwise the client could receive a refresh token that
	// the server does not know about.
	if err != nil {
		return nil, fmt.Errorf(
			"store refresh token: %w",
			err,
		)
	}

	// ---------------------------------------------------------
	// STEP 6: Return token pair
	// ---------------------------------------------------------

	// Return the RAW refresh token to the client.
	//
	// Remember:
	//
	// rawRefreshToken → client
	//
	// tokenHash → database
	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: rawRefreshToken,
	}, nil
}

// generateRefreshToken creates a cryptographically secure
// random refresh token.
//
// We use crypto/rand instead of math/rand.
//
// crypto/rand is designed for security-sensitive randomness.
//
// math/rand should NOT be used for authentication tokens.
func generateRefreshToken() (string, error) {

	// Allocate 32 random bytes.
	//
	// 32 bytes = 256 bits of randomness.
	//
	// That gives us an enormous number of possible tokens,
	// making brute-force guessing practically impossible.
	raw := make([]byte, 32)

	// Fill the byte slice using the operating system's
	// cryptographically secure random number generator.
	//
	// The random bytes are not predictable.
	_, err := rand.Read(raw)

	// If the operating system's secure random generator fails,
	// return the error.
	if err != nil {
		return "", err
	}

	// Convert the random bytes into hexadecimal.
	//
	// 32 random bytes become:
	//
	// 64 hexadecimal characters.
	//
	// Example:
	//
	// 9f7a2c4e...etc
	//
	// This makes the token safe to transmit as a string.
	return hex.EncodeToString(raw), nil
}

// hashRefreshToken hashes the raw refresh token.
//
// We use SHA-256:
//
//	raw token
//	    ↓
//	SHA-256
//	    ↓
//	64-character hexadecimal hash
//
// The hash is what gets stored in PostgreSQL.
func hashRefreshToken(rawToken string) string {

	// Calculate SHA-256 over the raw token bytes.
	hash := sha256.Sum256([]byte(rawToken))

	// Convert the 32-byte SHA-256 result into a
	// 64-character hexadecimal string.
	//
	// This exactly matches your database:
	//
	// token_hash VARCHAR(64)
	return hex.EncodeToString(hash[:])
}

// RefreshAccessToken takes a raw refresh token and rotates it.
//
// Flow:
//
//	Raw refresh token
//	       ↓
//	    SHA-256
//	       ↓
//	   DB lookup
//	       ↓
//	Check revoked
//	       ↓
//	Check expiry
//	       ↓
//	Generate new access token
//	       ↓
//	Generate new refresh token
//	       ↓
//	Revoke old + insert new
//	       ↓
//	Commit transaction
func (s *Service) RefreshAccessToken(
	ctx context.Context,
	rawRefreshToken string,
) (*TokenPair, error) {

	// A refresh token cannot be empty.
	if rawRefreshToken == "" {
		return nil, errors.New("refresh token is required")
	}

	// ---------------------------------------------------------
	// STEP 1: Hash the token received from the client
	// ---------------------------------------------------------

	// The client sends the RAW token.
	//
	// We must convert it into the same hash that was stored
	// in PostgreSQL.
	tokenHash := hashRefreshToken(rawRefreshToken)

	// ---------------------------------------------------------
	// STEP 2: Find the token in PostgreSQL
	// ---------------------------------------------------------

	// Look up the token using its SHA-256 hash.
	storedToken, err := s.refreshRepo.GetByHash(
		ctx,
		tokenHash,
	)

	// If the token does not exist, reject it.
	if err != nil {
		return nil, errors.New("invalid refresh token")
	}

	// ---------------------------------------------------------
	// STEP 3: Check whether token was revoked
	// ---------------------------------------------------------

	// A revoked token can NEVER be used again.
	//
	// This is especially important with rotation.
	//
	// Once:
	//
	//	OLD TOKEN → revoked
	//
	// another request using that old token must fail.
	if storedToken.Revoked {
		return nil, errors.New("refresh token has been revoked")
	}

	// ---------------------------------------------------------
	// STEP 4: Check expiration
	// ---------------------------------------------------------

	// If the current time is after ExpiresAt,
	// the refresh token is no longer valid.
	if time.Now().After(storedToken.ExpiresAt) {
		return nil, errors.New("refresh token has expired")
	}

	// ---------------------------------------------------------
	// STEP 5: Generate a NEW access token
	// ---------------------------------------------------------

	// Refresh reads the current persisted tier so a newly issued access token
	// reflects upgrades made after the previous access token was signed.
	tier, err := s.tierProvider.GetTier(ctx, storedToken.UserID)
	if err != nil {
		return nil, fmt.Errorf("get tier: %w", err)
	}

	// Generate a new short-lived access token.
	accessToken, err := s.jwtService.GenerateAccessToken(
		storedToken.UserID.String(),
		tier,
	)

	// Stop if access-token generation failed.
	if err != nil {
		return nil, fmt.Errorf(
			"generate new access token: %w",
			err,
		)
	}

	// ---------------------------------------------------------
	// STEP 6: Generate a NEW refresh token
	// ---------------------------------------------------------

	// Refresh-token rotation means we DO NOT reuse
	// the old refresh token.
	//
	// We generate a completely new one.
	newRawRefreshToken, err := generateRefreshToken()

	// Stop if secure random generation fails.
	if err != nil {
		return nil, fmt.Errorf(
			"generate new refresh token: %w",
			err,
		)
	}

	// Hash the new raw refresh token before storing it.
	newTokenHash := hashRefreshToken(newRawRefreshToken)

	// Capture creation time for the new token.
	now := time.Now()

	// Build the new database record.
	newRefreshRecord := &refresh.RefreshToken{
		TokenHash: newTokenHash,

		UserID: storedToken.UserID,

		// The new refresh token gets a fresh lifetime.
		ExpiresAt: now.Add(RefreshTokenTTL),

		// New token starts as active.
		Revoked: false,

		CreatedAt: now,
	}

	// ---------------------------------------------------------
	// STEP 7: Atomically rotate the refresh token
	// ---------------------------------------------------------

	// Rotate() performs:
	//
	//	BEGIN
	//	   ↓
	//	Revoke old token
	//	   ↓
	//	Insert new token
	//	   ↓
	//	COMMIT
	//
	// If either operation fails:
	//
	//	ROLLBACK
	//
	// This prevents a partially completed rotation.
	err = s.refreshRepo.Rotate(
		ctx,
		tokenHash,
		newRefreshRecord,
	)

	// If rotation failed, do not return the new token.
	if err != nil {
		return nil, fmt.Errorf(
			"rotate refresh token: %w",
			err,
		)
	}

	// ---------------------------------------------------------
	// STEP 8: Return the new token pair
	// ---------------------------------------------------------

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: newRawRefreshToken,
	}, nil
}
