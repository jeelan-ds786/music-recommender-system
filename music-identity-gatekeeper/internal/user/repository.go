package user

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	CreateUser(ctx context.Context, user *User) error
	GetByEmail(ctx context.Context, email string) (*User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
}

type PostgresRepository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) Repository {
	return &PostgresRepository{
		db: db,
	}
}

func (r *PostgresRepository) CreateUser(
	ctx context.Context,
	user *User,
) error {

	query := `
	INSERT INTO users (
		id,
		email,
		hashed_password,
		auth_provider
	)
	VALUES ($1, $2, $3, $4)
`

	var passwordHash any
	if user.HashedPassword != "" {
		passwordHash = user.HashedPassword
	}

	_, err := r.db.Exec(
		ctx,
		query,
		user.ID,
		user.Email,
		passwordHash,
		user.AuthProvider,
	)

	return err

}

func (r *PostgresRepository) GetByEmail(
	ctx context.Context,
	email string,
) (*User, error) {

	query := `
	SELECT
		id,
		email,
		hashed_password,
		auth_provider,
		created_at,
		updated_at
	FROM users
	WHERE email = $1
`

	var user User
	var passwordHash sql.NullString

	err := r.db.QueryRow(
		ctx,
		query,
		email,
	).Scan(
		&user.ID,
		&user.Email,
		&passwordHash,
		&user.AuthProvider,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrUserNotFound
		}

		return nil, err
	}
	if passwordHash.Valid {
		user.HashedPassword = passwordHash.String
	}

	return &user, nil

}

func (r *PostgresRepository) GetByID(
	ctx context.Context,
	id uuid.UUID,
) (*User, error) {

	query := `
	SELECT
		id,
		email,
		hashed_password,
		auth_provider,
		created_at,
		updated_at
	FROM users
	WHERE id = $1
`

	var user User
	var passwordHash sql.NullString

	err := r.db.QueryRow(
		ctx,
		query,
		id,
	).Scan(
		&user.ID,
		&user.Email,
		&passwordHash,
		&user.AuthProvider,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrUserNotFound
		}

		return nil, err
	}
	if passwordHash.Valid {
		user.HashedPassword = passwordHash.String
	}

	return &user, nil

}
