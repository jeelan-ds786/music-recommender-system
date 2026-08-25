package profile

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/user"
)

// foreignKeyViolation is the Postgres SQLSTATE for a foreign-key constraint
// violation. Used to detect a JWT whose user was deleted after issuance.
const foreignKeyViolation = "23503"

type PatchFields struct {
	DisplayName *string
	AvatarURL   *string
	Country     *string
	Language    *string
	BirthYear   *int16
}

type ProfileRepository interface {
	GetByUserID(ctx context.Context, userID uuid.UUID) (*Profile, error)
	// EnsureExists lazily creates the listener_profiles and preferences rows
	// for userID if they don't already exist. Both inserts run in one
	// transaction because the ticket requires a user to never end up with
	// one row but not the other; ON CONFLICT DO NOTHING makes it safe to
	// call from concurrent first requests for the same new user.
	EnsureExists(ctx context.Context, userID uuid.UUID) error
	Update(ctx context.Context, userID uuid.UUID, patch PatchFields) (*Profile, error)
}

type TierRepository interface {
	GetTier(ctx context.Context, userID uuid.UUID) (string, error)
	Upgrade(ctx context.Context, userID uuid.UUID, tier string) error
}

type PostgresRepository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) GetTier(ctx context.Context, userID uuid.UUID) (string, error) {
	const query = `
		SELECT COALESCE(
			(SELECT subscription_tier::text FROM listener_profiles WHERE user_id = $1),
			'free'
		)
	`
	var tier string
	if err := r.db.QueryRow(ctx, query, userID).Scan(&tier); err != nil {
		return "", err
	}
	return tier, nil
}

func (r *PostgresRepository) Upgrade(
	ctx context.Context,
	userID uuid.UUID,
	tier string,
) error {
	const query = `
		INSERT INTO listener_profiles (user_id, subscription_tier)
		VALUES ($1, $2)
		ON CONFLICT (user_id) DO UPDATE
		SET subscription_tier = EXCLUDED.subscription_tier, updated_at = NOW()
	`
	_, err := r.db.Exec(ctx, query, userID, tier)
	if err != nil {
		return err
	}
	return nil
}

func (r *PostgresRepository) GetByUserID(
	ctx context.Context,
	userID uuid.UUID,
) (*Profile, error) {

	query := `
		SELECT
			user_id,
			display_name,
			avatar_url,
			country,
			language,
			birth_year,
			subscription_tier,
			created_at,
			updated_at
		FROM listener_profiles
		WHERE user_id = $1
	`

	var p Profile

	err := r.db.QueryRow(
		ctx,
		query,
		userID,
	).Scan(
		&p.UserID,
		&p.DisplayName,
		&p.AvatarURL,
		&p.Country,
		&p.Language,
		&p.BirthYear,
		&p.SubscriptionTier,
		&p.CreatedAt,
		&p.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrProfileNotFound
		}

		return nil, err
	}

	return &p, nil
}

func (r *PostgresRepository) EnsureExists(
	ctx context.Context,
	userID uuid.UUID,
) error {

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	// A no-op after a successful Commit (returns pgx.ErrTxClosed, which is
	// expected and safe to discard); on any early return it rolls back the
	// half-applied inserts, which is the actual point of deferring it.
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(
		ctx,
		`INSERT INTO listener_profiles (user_id) VALUES ($1) ON CONFLICT (user_id) DO NOTHING`,
		userID,
	)
	if err != nil {
		return mapForeignKeyViolation(err)
	}

	_, err = tx.Exec(
		ctx,
		`INSERT INTO preferences (user_id) VALUES ($1) ON CONFLICT (user_id) DO NOTHING`,
		userID,
	)
	if err != nil {
		return mapForeignKeyViolation(err)
	}

	return tx.Commit(ctx)
}

func (r *PostgresRepository) Update(
	ctx context.Context,
	userID uuid.UUID,
	patch PatchFields,
) (*Profile, error) {

	query := `
		UPDATE listener_profiles
		SET
			display_name = COALESCE($1, display_name),
			avatar_url    = COALESCE($2, avatar_url),
			country       = COALESCE($3, country),
			language      = COALESCE($4, language),
			birth_year    = COALESCE($5, birth_year),
			updated_at    = NOW()
		WHERE user_id = $6
		RETURNING
			user_id,
			display_name,
			avatar_url,
			country,
			language,
			birth_year,
			subscription_tier,
			created_at,
			updated_at
	`

	var p Profile

	err := r.db.QueryRow(
		ctx,
		query,
		patch.DisplayName,
		patch.AvatarURL,
		patch.Country,
		patch.Language,
		patch.BirthYear,
		userID,
	).Scan(
		&p.UserID,
		&p.DisplayName,
		&p.AvatarURL,
		&p.Country,
		&p.Language,
		&p.BirthYear,
		&p.SubscriptionTier,
		&p.CreatedAt,
		&p.UpdatedAt,
	)

	if err != nil {
		// UPDATE matching zero rows isn't a Postgres error, so a missing
		// row surfaces here as pgx.ErrNoRows from the RETURNING scan, not
		// as a distinct "not found" signal — without this check, patching
		// a user whose listener_profiles row doesn't exist yet would
		// silently succeed with no effect.
		if err == pgx.ErrNoRows {
			return nil, ErrProfileNotFound
		}

		return nil, err
	}

	return &p, nil
}

func mapForeignKeyViolation(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == foreignKeyViolation {
		return user.ErrUserNotFound
	}

	return err
}
