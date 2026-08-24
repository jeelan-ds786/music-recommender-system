package profile

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

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
