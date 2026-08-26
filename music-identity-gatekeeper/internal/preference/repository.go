package preference

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// OnboardingFields is the write model for CompleteOnboarding. GenreSeeds
// and LanguagePrefs replace the existing arrays outright (onboarding is a
// one-time initial setup, not a patch); FollowedArtistIDs is optional and
// additive against followed_artists / followed_artist_ids.
type OnboardingFields struct {
	GenreSeeds        []string
	LanguagePrefs     []string
	FollowedArtistIDs []uuid.UUID
}

type Repository interface {
	Create(ctx context.Context, userID uuid.UUID) error
	GetByUserID(ctx context.Context, userID uuid.UUID) (*Preference, error)
	LikeSong(ctx context.Context, userID, songID uuid.UUID) error
	UnlikeSong(ctx context.Context, userID, songID uuid.UUID) error
	ListLikedSongs(ctx context.Context, userID uuid.UUID, cursor *Cursor, limit int) ([]LikedSong, *Cursor, error)
	FollowArtist(ctx context.Context, userID, artistID uuid.UUID) error
	UnfollowArtist(ctx context.Context, userID, artistID uuid.UUID) error
	CompleteOnboarding(ctx context.Context, userID uuid.UUID, fields OnboardingFields) error
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

// ensurePreferencesRow guarantees a preferences row exists for userID
// before a dual-write touches its array columns. Without this, liking a
// song (or onboarding) as a user's very first action — before they've ever
// called GET/PATCH /me, which is what normally lazily creates this row —
// would insert into liked_songs but silently skip the array_append below
// (its WHERE clause can't match a row that doesn't exist yet), leaving the
// two stores permanently out of sync for that entry.
func ensurePreferencesRow(ctx context.Context, tx pgx.Tx, userID uuid.UUID) error {
	_, err := tx.Exec(
		ctx,
		`INSERT INTO preferences (user_id) VALUES ($1) ON CONFLICT (user_id) DO NOTHING`,
		userID,
	)

	return err
}

func (r *PostgresRepository) LikeSong(
	ctx context.Context,
	userID uuid.UUID,
	songID uuid.UUID,
) error {

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := ensurePreferencesRow(ctx, tx, userID); err != nil {
		return err
	}

	_, err = tx.Exec(
		ctx,
		`INSERT INTO liked_songs (user_id, song_id) VALUES ($1, $2) ON CONFLICT (user_id, song_id) DO NOTHING`,
		userID,
		songID,
	)
	if err != nil {
		return err
	}

	_, err = tx.Exec(
		ctx,
		`UPDATE preferences
			SET liked_song_ids = array_append(liked_song_ids, $1), updated_at = NOW()
			WHERE user_id = $2 AND NOT ($1 = ANY(liked_song_ids))`,
		songID,
		userID,
	)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *PostgresRepository) UnlikeSong(
	ctx context.Context,
	userID uuid.UUID,
	songID uuid.UUID,
) error {

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(
		ctx,
		`DELETE FROM liked_songs WHERE user_id = $1 AND song_id = $2`,
		userID,
		songID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrLikeNotFound
	}

	_, err = tx.Exec(
		ctx,
		`UPDATE preferences
			SET liked_song_ids = array_remove(liked_song_ids, $1), updated_at = NOW()
			WHERE user_id = $2`,
		songID,
		userID,
	)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// ListLikedSongs fetches one row beyond limit to detect whether a next
// page exists without a separate COUNT query; that extra row is trimmed
// before returning and its presence is what next drives the returned
// cursor.
func (r *PostgresRepository) ListLikedSongs(
	ctx context.Context,
	userID uuid.UUID,
	cursor *Cursor,
	limit int,
) ([]LikedSong, *Cursor, error) {

	// The service layer already clamps limit before calling here, but this
	// repository method is itself exported and callable directly (e.g. from
	// a future gRPC handler or a test), so it must not trust an unbounded
	// caller-supplied value when sizing the query LIMIT or the slice
	// pre-allocation below.
	if limit <= 0 {
		limit = defaultLikedSongsLimit
	}
	if limit > maxLikedSongsLimit {
		limit = maxLikedSongsLimit
	}

	var rows pgx.Rows
	var err error

	if cursor != nil {
		rows, err = r.db.Query(
			ctx,
			`SELECT song_id, created_at FROM liked_songs
				WHERE user_id = $1 AND (created_at, song_id) < ($2, $3)
				ORDER BY created_at DESC, song_id DESC
				LIMIT $4`,
			userID,
			cursor.CreatedAt,
			cursor.ID,
			limit+1,
		)
	} else {
		rows, err = r.db.Query(
			ctx,
			`SELECT song_id, created_at FROM liked_songs
				WHERE user_id = $1
				ORDER BY created_at DESC, song_id DESC
				LIMIT $2`,
			userID,
			limit+1,
		)
	}
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	// Capacity is the fixed upper bound (limit is clamped to
	// maxLikedSongsLimit above, and at most limit+1 rows are fetched), not
	// the caller-supplied limit itself — CodeQL's excessive-allocation
	// check flags any size expression that still references a
	// user-influenced value, even one already clamped via reassignment.
	items := make([]LikedSong, 0, maxLikedSongsLimit+1)
	for rows.Next() {
		var item LikedSong
		if err := rows.Scan(&item.SongID, &item.CreatedAt); err != nil {
			return nil, nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	var next *Cursor
	if len(items) > limit {
		items = items[:limit]
		last := items[limit-1]
		next = &Cursor{CreatedAt: last.CreatedAt, ID: last.SongID}
	}

	return items, next, nil
}

func (r *PostgresRepository) FollowArtist(
	ctx context.Context,
	userID uuid.UUID,
	artistID uuid.UUID,
) error {

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := ensurePreferencesRow(ctx, tx, userID); err != nil {
		return err
	}

	if err := followArtistTx(ctx, tx, userID, artistID); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// followArtistTx is shared by FollowArtist and CompleteOnboarding, which
// both need to add a followed artist inside a transaction they already
// own (CompleteOnboarding also needs ensurePreferencesRow, but it must run
// that once for the whole onboarding write, not once per artist).
func followArtistTx(ctx context.Context, tx pgx.Tx, userID, artistID uuid.UUID) error {
	_, err := tx.Exec(
		ctx,
		`INSERT INTO followed_artists (user_id, artist_id) VALUES ($1, $2) ON CONFLICT (user_id, artist_id) DO NOTHING`,
		userID,
		artistID,
	)
	if err != nil {
		return err
	}

	_, err = tx.Exec(
		ctx,
		`UPDATE preferences
			SET followed_artist_ids = array_append(followed_artist_ids, $1), updated_at = NOW()
			WHERE user_id = $2 AND NOT ($1 = ANY(followed_artist_ids))`,
		artistID,
		userID,
	)

	return err
}

func (r *PostgresRepository) UnfollowArtist(
	ctx context.Context,
	userID uuid.UUID,
	artistID uuid.UUID,
) error {

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(
		ctx,
		`DELETE FROM followed_artists WHERE user_id = $1 AND artist_id = $2`,
		userID,
		artistID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrFollowNotFound
	}

	_, err = tx.Exec(
		ctx,
		`UPDATE preferences
			SET followed_artist_ids = array_remove(followed_artist_ids, $1), updated_at = NOW()
			WHERE user_id = $2`,
		artistID,
		userID,
	)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *PostgresRepository) CompleteOnboarding(
	ctx context.Context,
	userID uuid.UUID,
	fields OnboardingFields,
) error {

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := ensurePreferencesRow(ctx, tx, userID); err != nil {
		return err
	}

	// FOR UPDATE row-locks the preferences row for the rest of this
	// transaction, so two concurrent onboarding submissions for the same
	// user serialize here: the second blocks until the first commits, then
	// re-reads onboarded_at and correctly sees it already set.
	var onboardedAt *time.Time
	err = tx.QueryRow(
		ctx,
		`SELECT onboarded_at FROM preferences WHERE user_id = $1 FOR UPDATE`,
		userID,
	).Scan(&onboardedAt)
	if err != nil {
		return err
	}
	if onboardedAt != nil {
		return ErrOnboardingAlreadyCompleted
	}

	_, err = tx.Exec(
		ctx,
		`UPDATE preferences
			SET genre_seeds = $1, language_prefs = $2, onboarded_at = NOW(), updated_at = NOW()
			WHERE user_id = $3`,
		fields.GenreSeeds,
		fields.LanguagePrefs,
		userID,
	)
	if err != nil {
		return err
	}

	for _, artistID := range fields.FollowedArtistIDs {
		if err := followArtistTx(ctx, tx, userID, artistID); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}
