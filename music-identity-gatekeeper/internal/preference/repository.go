package preference

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	Create(ctx context.Context, userID uuid.UUID) error
	GetByUserID(ctx context.Context, userID uuid.UUID) (*Preference, error)
}

type PostgresRepository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) Repository {
	return &PostgresRepository{
		db: db,
	}
}

func (r *PostgresRepository) Create(
	ctx context.Context,
	userID uuid.UUID,
) error {

	query := `
		INSERT INTO preferences (user_id)
		VALUES ($1)
	`

	_, err := r.db.Exec(
		ctx,
		query,
		userID,
	)

	return err
}

func (r *PostgresRepository) GetByUserID(
	ctx context.Context,
	userID uuid.UUID,
) (*Preference, error) {

	query := `
		SELECT
			user_id,
			liked_song_ids,
			followed_artist_ids,
			genre_seeds,
			language_prefs,
			updated_at
		FROM preferences
		WHERE user_id = $1
	`

	var p Preference

	err := r.db.QueryRow(
		ctx,
		query,
		userID,
	).Scan(
		&p.UserID,
		&p.LikedSongIDs,
		&p.FollowedArtistIDs,
		&p.GenreSeeds,
		&p.LanguagePrefs,
		&p.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrPreferenceNotFound
		}

		return nil, err
	}

	return &p, nil
}
