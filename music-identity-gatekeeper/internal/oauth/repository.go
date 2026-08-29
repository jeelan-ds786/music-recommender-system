package oauth

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/user"
)

type PostgresAccountRepository struct {
	db *pgxpool.Pool
}

func NewAccountRepository(db *pgxpool.Pool) *PostgresAccountRepository {
	return &PostgresAccountRepository{db: db}
}

func (r *PostgresAccountRepository) GetUser(
	ctx context.Context,
	provider string,
	providerSubject string,
) (*user.User, error) {
	const query = `
		SELECT u.id, u.email, u.hashed_password, u.auth_provider, u.created_at, u.updated_at
		FROM oauth_accounts oa
		JOIN users u ON u.id = oa.user_id
		WHERE oa.provider = $1 AND oa.provider_subject = $2
	`
	var result user.User
	var passwordHash sql.NullString
	err := r.db.QueryRow(ctx, query, provider, providerSubject).Scan(
		&result.ID,
		&result.Email,
		&passwordHash,
		&result.AuthProvider,
		&result.CreatedAt,
		&result.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, user.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	if passwordHash.Valid {
		result.HashedPassword = passwordHash.String
	}
	return &result, nil
}

func (r *PostgresAccountRepository) CreateUser(
	ctx context.Context,
	newUser *user.User,
	provider string,
	providerSubject string,
) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const userQuery = `
		INSERT INTO users (id, email, hashed_password, auth_provider)
		VALUES ($1, $2, NULL, $3)
	`
	if _, err := tx.Exec(ctx, userQuery, newUser.ID, newUser.Email, provider); err != nil {
		return err
	}

	const accountQuery = `
		INSERT INTO oauth_accounts (provider, provider_subject, user_id, provider_email)
		VALUES ($1, $2, $3, $4)
	`
	if _, err := tx.Exec(ctx, accountQuery, provider, providerSubject, newUser.ID, newUser.Email); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
