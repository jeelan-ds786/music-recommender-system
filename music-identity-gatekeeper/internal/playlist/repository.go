package playlist

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PatchFields is the write model for Update. A nil field leaves the
// corresponding column unchanged, mirroring profile.PatchFields.
type PatchFields struct {
	Name        *string
	Description *string
	IsPublic    *bool
}

type Repository interface {
	Create(ctx context.Context, userID uuid.UUID, name string, description *string, isPublic bool) (*Playlist, error)
	GetByID(ctx context.Context, playlistID, userID uuid.UUID) (*Playlist, error)
	ListByUser(ctx context.Context, userID uuid.UUID) ([]Playlist, error)
	Update(ctx context.Context, playlistID, userID uuid.UUID, patch PatchFields) (*Playlist, error)
	Delete(ctx context.Context, playlistID, userID uuid.UUID) error
	AddSong(ctx context.Context, playlistID, userID, songID uuid.UUID) error
	RemoveSong(ctx context.Context, playlistID, userID, songID uuid.UUID) error
	ListSongs(ctx context.Context, playlistID, userID uuid.UUID) ([]PlaylistSong, error)
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
	name string,
	description *string,
	isPublic bool,
) (*Playlist, error) {

	query := `
		INSERT INTO playlists (user_id, name, description, is_public)
		VALUES ($1, $2, $3, $4)
		RETURNING id, user_id, name, description, is_public, created_at, updated_at
	`

	var p Playlist

	err := r.db.QueryRow(
		ctx,
		query,
		userID,
		name,
		description,
		isPublic,
	).Scan(
		&p.ID,
		&p.UserID,
		&p.Name,
		&p.Description,
		&p.IsPublic,
		&p.CreatedAt,
		&p.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &p, nil
}

func (r *PostgresRepository) GetByID(
	ctx context.Context,
	playlistID uuid.UUID,
	userID uuid.UUID,
) (*Playlist, error) {

	query := `
		SELECT id, user_id, name, description, is_public, created_at, updated_at
		FROM playlists
		WHERE id = $1 AND user_id = $2
	`

	var p Playlist

	err := r.db.QueryRow(
		ctx,
		query,
		playlistID,
		userID,
	).Scan(
		&p.ID,
		&p.UserID,
		&p.Name,
		&p.Description,
		&p.IsPublic,
		&p.CreatedAt,
		&p.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrPlaylistNotFound
		}

		return nil, err
	}

	return &p, nil
}

func (r *PostgresRepository) ListByUser(
	ctx context.Context,
	userID uuid.UUID,
) ([]Playlist, error) {

	query := `
		SELECT id, user_id, name, description, is_public, created_at, updated_at
		FROM playlists
		WHERE user_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	playlists := make([]Playlist, 0)
	for rows.Next() {
		var p Playlist
		if err := rows.Scan(
			&p.ID,
			&p.UserID,
			&p.Name,
			&p.Description,
			&p.IsPublic,
			&p.CreatedAt,
			&p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		playlists = append(playlists, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return playlists, nil
}

func (r *PostgresRepository) Update(
	ctx context.Context,
	playlistID uuid.UUID,
	userID uuid.UUID,
	patch PatchFields,
) (*Playlist, error) {

	query := `
		UPDATE playlists
		SET
			name        = COALESCE($1, name),
			description = COALESCE($2, description),
			is_public   = COALESCE($3, is_public),
			updated_at  = NOW()
		WHERE id = $4 AND user_id = $5
		RETURNING id, user_id, name, description, is_public, created_at, updated_at
	`

	var p Playlist

	err := r.db.QueryRow(
		ctx,
		query,
		patch.Name,
		patch.Description,
		patch.IsPublic,
		playlistID,
		userID,
	).Scan(
		&p.ID,
		&p.UserID,
		&p.Name,
		&p.Description,
		&p.IsPublic,
		&p.CreatedAt,
		&p.UpdatedAt,
	)

	if err != nil {
		// UPDATE matching zero rows isn't a Postgres error, so a missing or
		// not-owned playlist surfaces here as pgx.ErrNoRows from the
		// RETURNING scan, not as a distinct "not found" signal — same as
		// profile.PostgresRepository.Update.
		if err == pgx.ErrNoRows {
			return nil, ErrPlaylistNotFound
		}

		return nil, err
	}

	return &p, nil
}

func (r *PostgresRepository) Delete(
	ctx context.Context,
	playlistID uuid.UUID,
	userID uuid.UUID,
) error {

	tag, err := r.db.Exec(
		ctx,
		`DELETE FROM playlists WHERE id = $1 AND user_id = $2`,
		playlistID,
		userID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrPlaylistNotFound
	}

	return nil
}

// AddSong is idempotent and race-safe: SELECT ... FOR UPDATE on the parent
// playlists row serializes concurrent writers to the same playlist (without
// blocking writers to other playlists) so the "next position" computed
// below can't collide, the same pattern preference.PostgresRepository.
// CompleteOnboarding uses to serialize onboarding writes.
func (r *PostgresRepository) AddSong(
	ctx context.Context,
	playlistID uuid.UUID,
	userID uuid.UUID,
	songID uuid.UUID,
) error {

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var exists bool
	err = tx.QueryRow(
		ctx,
		`SELECT EXISTS(SELECT 1 FROM playlists WHERE id = $1 AND user_id = $2 FOR UPDATE)`,
		playlistID,
		userID,
	).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return ErrPlaylistNotFound
	}

	var already bool
	err = tx.QueryRow(
		ctx,
		`SELECT EXISTS(SELECT 1 FROM playlist_songs WHERE playlist_id = $1 AND song_id = $2)`,
		playlistID,
		songID,
	).Scan(&already)
	if err != nil {
		return err
	}
	if already {
		return tx.Commit(ctx)
	}

	var next int
	err = tx.QueryRow(
		ctx,
		`SELECT COALESCE(MAX(position), -1) + 1 FROM playlist_songs WHERE playlist_id = $1`,
		playlistID,
	).Scan(&next)
	if err != nil {
		return err
	}

	_, err = tx.Exec(
		ctx,
		`INSERT INTO playlist_songs (playlist_id, song_id, position) VALUES ($1, $2, $3)`,
		playlistID,
		songID,
		next,
	)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// RemoveSong scopes the DELETE through playlists.user_id so a playlist that
// doesn't exist or isn't the caller's collapses into the same
// ErrSongNotInPlaylist as a song that was never added — no separate
// ownership round trip, same anti-enumeration approach used elsewhere in
// this service. No position resequencing: gaps are harmless since ordering
// only ever reads ORDER BY position.
func (r *PostgresRepository) RemoveSong(
	ctx context.Context,
	playlistID uuid.UUID,
	userID uuid.UUID,
	songID uuid.UUID,
) error {

	tag, err := r.db.Exec(
		ctx,
		`DELETE FROM playlist_songs
			USING playlists
			WHERE playlist_songs.playlist_id = playlists.id
				AND playlists.id = $1
				AND playlists.user_id = $2
				AND playlist_songs.song_id = $3`,
		playlistID,
		userID,
		songID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrSongNotInPlaylist
	}

	return nil
}

func (r *PostgresRepository) ListSongs(
	ctx context.Context,
	playlistID uuid.UUID,
	userID uuid.UUID,
) ([]PlaylistSong, error) {

	query := `
		SELECT ps.song_id, ps.position, ps.added_at
		FROM playlist_songs ps
		JOIN playlists p ON p.id = ps.playlist_id
		WHERE p.id = $1 AND p.user_id = $2
		ORDER BY ps.position
	`

	rows, err := r.db.Query(ctx, query, playlistID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	songs := make([]PlaylistSong, 0)
	for rows.Next() {
		var s PlaylistSong
		if err := rows.Scan(&s.SongID, &s.Position, &s.AddedAt); err != nil {
			return nil, err
		}
		songs = append(songs, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return songs, nil
}
