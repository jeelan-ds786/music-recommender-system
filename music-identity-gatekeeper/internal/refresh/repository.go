package refresh

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository handles all database operations related to refresh tokens.

// The repository is responsible ONLY for persistence.

// It should NOT:
// - generate refresh tokens
// - hash refresh tokens
// - validate JWTs
// - decide authentication rules

// Those responsibilities belong to the token/service layer.
//
// Repository responsibility:
//
//	Go object
//	    ↓
//	SQL query
//	    ↓
//	PostgreSQL
type Repository struct {

	// db is the database connection pool.
	//
	// *sql.DB is safe for concurrent use and manages a pool
	// of database connections internally.
	//
	// We keep it inside the repository so all refresh-token
	// database operations use the same database dependency.
	db *pgxpool.Pool
}

// NewRepository creates a new refresh-token repository.
//
// We use dependency injection here.
//
// Instead of creating a database connection inside the repository,
// the caller gives us an existing *sql.DB.
func NewRepository(db *pgxpool.Pool) *Repository {

	// Return a Repository containing the database connection.
	return &Repository{
		db: db,
	}
}

// CreateRefreshToken inserts a new refresh-token record
// into the refresh_tokens table.
//
// The raw refresh token NEVER reaches this method.
//
// The caller should first hash the raw token:
//
//	raw token
//	    ↓
//	SHA-256
//	    ↓
//	tokenHash
//
// Then only tokenHash is stored in PostgreSQL.
func (r *Repository) CreateRefreshToken(
	ctx context.Context,
	token *RefreshToken,
) error {

	// SQL INSERT statement.
	//
	// We insert these five columns:
	//
	// token_hash
	// user_id
	// expires_at
	// revoked
	// created_at
	//
	// PostgreSQL placeholders use:
	//
	// $1
	// $2
	// $3
	// $4
	// $5
	//
	// This is parameterized SQL.
	// It protects us from SQL injection and lets PostgreSQL
	// handle the values separately from the SQL statement.
	const query = `
		INSERT INTO refresh_tokens (
			token_hash,
			user_id,
			expires_at,
			revoked,
			created_at
		)
		VALUES ($1, $2, $3, $4, $5)
	`

	// Execute the INSERT query.
	//
	// ExecContext is used because:
	//
	// 1. It accepts context.Context.
	// 2. The request can be cancelled.
	// 3. Database work can respect request deadlines.
	//
	// The values are mapped as:
	//
	// $1 → token.TokenHash
	// $2 → token.UserID
	// $3 → token.ExpiresAt
	// $4 → token.Revoked
	// $5 → token.CreatedAt
	_, err := r.db.Exec(
		ctx,
		query,
		token.TokenHash,
		token.UserID,
		token.ExpiresAt,
		token.Revoked,
		token.CreatedAt,
	)

	// If PostgreSQL returned an error, return it to the
	// service layer.
	if err != nil {
		return err
	}

	// nil means the database operation succeeded.
	return nil
}

// GetByHash retrieves a refresh token using its SHA-256 hash.
//
// This is the lookup performed when the client sends:
//
//	POST /auth/refresh
//
//	{
//	    "refresh_token": "raw-token"
//	}
//
// The service will:
//
//	raw token
//	    ↓
//	SHA-256
//	    ↓
//	hash
//	    ↓
//	GetByHash(hash)
func (r *Repository) GetByHash(
	ctx context.Context,
	tokenHash string,
) (*RefreshToken, error) {

	// SQL SELECT statement.
	//
	// We retrieve all columns needed to validate the refresh token.
	const query = `
		SELECT
			token_hash,
			user_id,
			expires_at,
			revoked,
			created_at
		FROM refresh_tokens
		WHERE token_hash = $1
	`

	// Create an empty RefreshToken struct.
	//
	// Scan() will populate this struct with values returned
	// from PostgreSQL.
	token := &RefreshToken{}

	// QueryRowContext executes a query that should return
	// exactly one row.
	//
	// We use QueryRow because token_hash is the PRIMARY KEY.
	//
	// Therefore:
	//
	// one hash → maximum one database row.
	row := r.db.QueryRow(
		ctx,
		query,
		tokenHash,
	)

	// Scan copies database columns into Go variables.
	//
	// The order MUST match the SELECT order:
	//
	// SELECT token_hash  → token.TokenHash
	// SELECT user_id     → token.UserID
	// SELECT expires_at  → token.ExpiresAt
	// SELECT revoked     → token.Revoked
	// SELECT created_at  → token.CreatedAt
	err := row.Scan(
		&token.TokenHash,
		&token.UserID,
		&token.ExpiresAt,
		&token.Revoked,
		&token.CreatedAt,
	)

	// sql.ErrNoRows means the hash does not exist.
	//
	// We return it unchanged so the service layer can decide
	// how to convert it into an authentication error.
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, pgx.ErrNoRows
	}

	// Any other database error should also be returned.
	if err != nil {
		return nil, err
	}

	// Return the populated refresh token.
	return token, nil
}

// Revoke marks a refresh token as revoked.
//
// We DO NOT delete the token.
//
// Instead:
//
//	revoked = false
//	       ↓
//	revoked = true
//
// Keeping the record allows us to maintain token history
// and prevents the old token from becoming usable again.
func (r *Repository) Revoke(
	ctx context.Context,
	tokenHash string,
) error {

	// UPDATE changes only the revoked flag.
	//
	// WHERE token_hash = $1 ensures that only the requested
	// refresh token is modified.
	const query = `
		UPDATE refresh_tokens
		SET revoked = true
		WHERE token_hash = $1
	`

	// Execute the UPDATE.
	result, err := r.db.Exec(
		ctx,
		query,
		tokenHash,
	)

	// Database error.
	if err != nil {
		return err
	}

	// Check how many rows were modified.
	rowsAffected := result.RowsAffected()

	// If zero rows were affected, the token does not exist.
	//
	// This is useful because it prevents us from silently
	// pretending that a nonexistent token was revoked.
	if rowsAffected == 0 {
		return pgx.ErrNoRows
	}

	// Token successfully revoked.
	return nil
}

// Rotate performs refresh-token rotation atomically.
//
// Rotation means:
//
//	OLD TOKEN
//	    ↓
//	Revoke old token
//
//	NEW TOKEN
//	    ↓
//	Insert new token
//
// Both operations must succeed.
//
// If revocation succeeds but insertion fails,
// we DON'T want the database to permanently remain
// in a partially completed state.
//
// Therefore we use a transaction:
//
//	BEGIN
//	   ↓
//	REVOKE OLD
//	   ↓
//	INSERT NEW
//	   ↓
//	COMMIT
//
// If anything fails:
//
//	ROLLBACK
func (r *Repository) Rotate(
	ctx context.Context,
	oldTokenHash string,
	newToken *RefreshToken,
) error {

	// Start a database transaction.
	//
	// From this point onward, changes belong to this transaction
	// until Commit() or Rollback() is called.
	tx, err := r.db.BeginTx(
		ctx,
		pgx.TxOptions{},
	)

	// If the transaction could not start, return the error.
	if err != nil {
		return err
	}

	// Rollback is safe even if Commit() eventually succeeds.
	//
	// This deferred function protects us if an error occurs
	// anywhere before Commit().
	//
	// After a successful Commit(), Rollback() simply has
	// nothing left to roll back.
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	// ---------------------------------------------------------
	// STEP 1: Revoke the old refresh token
	// ---------------------------------------------------------

	const revokeQuery = `
		UPDATE refresh_tokens
		SET revoked = true
		WHERE token_hash = $1
		  AND revoked = false
	`

	// Execute the revoke operation inside the transaction.
	result, err := tx.Exec(
		ctx,
		revokeQuery,
		oldTokenHash,
	)

	// If the UPDATE failed, rollback will happen through
	// the deferred tx.Rollback().
	if err != nil {
		return err
	}

	// Check whether the old token was actually revoked.
	rowsAffected := result.RowsAffected()

	// If zero rows were affected, one of these is true:
	//
	// 1. The token doesn't exist.
	// 2. The token was already revoked.
	//
	// In either case we must NOT issue a new refresh token.
	if rowsAffected == 0 {
		return pgx.ErrNoRows
	}

	// ---------------------------------------------------------
	// STEP 2: Insert the new refresh token
	// ---------------------------------------------------------

	const insertQuery = `
		INSERT INTO refresh_tokens (
			token_hash,
			user_id,
			expires_at,
			revoked,
			created_at
		)
		VALUES ($1, $2, $3, $4, $5)
	`

	// Insert the new token inside the SAME transaction.
	//
	// Notice that we are NOT inserting the raw token.
	// Only its hash is stored.
	_, err = tx.Exec(
		ctx,
		insertQuery,
		newToken.TokenHash,
		newToken.UserID,
		newToken.ExpiresAt,
		newToken.Revoked,
		newToken.CreatedAt,
	)

	// If insertion fails:
	//
	// deferred Rollback()
	//        ↓
	// old token revocation is also undone.
	//
	// Therefore we don't end up with an invalid authentication
	// state where the old token is revoked but the new token
	// was never created.
	if err != nil {
		return err
	}

	// ---------------------------------------------------------
	// STEP 3: Commit transaction
	// ---------------------------------------------------------

	// Commit permanently applies BOTH operations:
	//
	// 1. old token → revoked = true
	// 2. new token → inserted
	//
	// If Commit succeeds, rotation is complete.
	if err := tx.Commit(ctx); err != nil {
		return err
	}

	// Everything succeeded.
	return nil
}

// DeleteExpired is an optional maintenance method.
//
// It is NOT required for authentication.
//
// The revoked flag protects us from using revoked tokens,
// and expires_at protects us from using expired tokens.
//
// However, expired rows can accumulate over time.
//
// A background cleanup job can periodically delete old
// expired/revoked tokens.
func (r *Repository) DeleteExpired(
	ctx context.Context,
	before time.Time,
) error {

	// Delete tokens that are both:
	//
	// 1. Expired
	// 2. Revoked
	//
	// Keeping the condition restrictive prevents us from
	// accidentally deleting an expired token that might still
	// be useful for auditing.
	const query = `
		DELETE FROM refresh_tokens
		WHERE expires_at < $1
		  AND revoked = true
	`

	_, err := r.db.Exec(
		ctx,
		query,
		before,
	)

	return err
}

// DeleteByUser removes all refresh tokens belonging to a user.
//
// This can be useful for:
//
// - Logout from all devices
// - Password change
// - Account compromise
// - Admin-forced logout
func (r *Repository) DeleteByUser(
	ctx context.Context,
	userID uuid.UUID,
) error {

	const query = `
		DELETE FROM refresh_tokens
		WHERE user_id = $1
	`

	_, err := r.db.Exec(
		ctx,
		query,
		userID,
	)

	return err
}
