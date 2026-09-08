package event

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Outbox interface {
	MarkPublished(context.Context, uuid.UUID) error
	RecordFailure(context.Context, uuid.UUID, string) error
	FetchPending(context.Context, int, int) ([]Message, error)
}

type PostgresOutbox struct {
	db *pgxpool.Pool
}

func NewOutbox(db *pgxpool.Pool) *PostgresOutbox {
	return &PostgresOutbox{db: db}
}

func (o *PostgresOutbox) MarkPublished(ctx context.Context, id uuid.UUID) error {
	_, err := o.db.Exec(ctx, `
		UPDATE outbox_events
		SET published_at = NOW(), last_error = NULL
		WHERE id = $1 AND published_at IS NULL
	`, id)
	return err
}

func (o *PostgresOutbox) RecordFailure(ctx context.Context, id uuid.UUID, failure string) error {
	_, err := o.db.Exec(ctx, `
		UPDATE outbox_events
		SET attempts = attempts + 1, last_error = LEFT($2, 1024)
		WHERE id = $1 AND published_at IS NULL
	`, id, failure)
	return err
}

func (o *PostgresOutbox) FetchPending(ctx context.Context, limit, maxAttempts int) ([]Message, error) {
	rows, err := o.db.Query(ctx, `
		SELECT id, playback_event_id, topic, message_key, event_type,
		       schema_version, headers, payload, occurred_at, created_at,
		       published_at, attempts, COALESCE(last_error, '')
		FROM outbox_events
		WHERE published_at IS NULL AND attempts < $2
		ORDER BY created_at, id
		LIMIT $1
	`, limit, maxAttempts)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages := make([]Message, 0)
	for rows.Next() {
		var message Message
		var headers []byte
		if err := rows.Scan(
			&message.ID, &message.PlaybackEventID, &message.Topic, &message.Key,
			&message.EventType, &message.SchemaVersion, &headers, &message.Payload,
			&message.OccurredAt, &message.CreatedAt, &message.PublishedAt,
			&message.Attempts, &message.LastError,
		); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(headers, &message.Headers); err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}
	return messages, rows.Err()
}

var _ Outbox = (*PostgresOutbox)(nil)
