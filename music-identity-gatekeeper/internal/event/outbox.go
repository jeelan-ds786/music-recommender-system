package event

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/logger"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/reqid"
)

type Outbox struct {
	pool *pgxpool.Pool
	log  *logger.Logger
}

func NewOutbox(pool *pgxpool.Pool, log *logger.Logger) *Outbox {
	return &Outbox{
		pool: pool,
		log:  log,
	}
}

type OutboxEvent struct {
	ID        int64
	Topic     string
	Key       string
	Payload   []byte
	Status    string
	Attempts  int
	CreatedAt time.Time
}

// Enqueue inserts a pending event row and returns its ID so the caller can
// immediately attempt a direct publish and update this same row's status
// (MarkPublished/IncrementAttempts) without a second lookup.
func (o *Outbox) Enqueue(ctx context.Context, tx pgx.Tx, topic, key string, payload []byte) (int64, error) {
	query := `
		INSERT INTO kafka_integration (topic, key, payload)
		VALUES ($1, $2, $3)
		RETURNING id
	`
	args := []any{topic, key, payload}

	var id int64
	var err error
	if tx != nil {
		err = tx.QueryRow(ctx, query, args...).Scan(&id)
	} else {
		err = o.pool.QueryRow(ctx, query, args...).Scan(&id)
	}

	if err != nil {
		rid, _ := reqid.FromContext(ctx)
		o.log.Error(rid, "failed to enqueue outbox event for topic=%s key=%s: %v", topic, key, err)
		return 0, err
	}

	return id, nil
}

// GetByID fetches a single row regardless of status — useful for
// verification/debugging and for tests that need to check a row's status
// after a direct-publish attempt (FetchPending only returns pending rows).
func (o *Outbox) GetByID(ctx context.Context, id int64) (*OutboxEvent, error) {
	query := `
		SELECT id, topic, key, payload, status, attempts, created_at
		FROM kafka_integration
		WHERE id = $1
	`

	var e OutboxEvent
	err := o.pool.QueryRow(ctx, query, id).Scan(
		&e.ID, &e.Topic, &e.Key, &e.Payload, &e.Status, &e.Attempts, &e.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &e, nil
}

func (o *Outbox) FetchPending(ctx context.Context, limit int) ([]OutboxEvent, error) {
	query := `
		SELECT id, topic, key, payload, status, attempts, created_at
		FROM kafka_integration
		WHERE status = 'pending'
		ORDER BY created_at ASC
		LIMIT $1
	`

	rows, err := o.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []OutboxEvent
	for rows.Next() {
		var e OutboxEvent
		if err := rows.Scan(&e.ID, &e.Topic, &e.Key, &e.Payload, &e.Status, &e.Attempts, &e.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, e)
	}

	return events, rows.Err()
}

func (o *Outbox) MarkPublished(ctx context.Context, id int64) error {
	query := `
		UPDATE kafka_integration
		SET status = 'published', published_at = NOW()
		WHERE id = $1
	`
	_, err := o.pool.Exec(ctx, query, id)
	return err
}

func (o *Outbox) MarkFailed(ctx context.Context, id int64) error {
	query := `
		UPDATE kafka_integration
		SET status = 'failed'
		WHERE id = $1
	`
	_, err := o.pool.Exec(ctx, query, id)
	return err
}

func (o *Outbox) IncrementAttempts(ctx context.Context, id int64) error {
	query := `
		UPDATE kafka_integration
		SET attempts = attempts + 1
		WHERE id = $1
	`
	_, err := o.pool.Exec(ctx, query, id)
	return err
}
