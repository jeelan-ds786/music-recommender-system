package idempotency

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jeelan-ds786/music-recommender-system/music-event-streaming-service/internal/consumer"
)

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) Claim(ctx context.Context, eventID, handlerName string) (consumer.ClaimState, error) {
	var state string
	err := s.pool.QueryRow(ctx, `
		INSERT INTO processed_events (event_id, handler_name, state, lease_until)
		VALUES ($1, $2, 'processing', NOW() + INTERVAL '5 minutes')
		ON CONFLICT (event_id, handler_name) DO UPDATE
		SET lease_until = EXCLUDED.lease_until
		WHERE processed_events.state = 'processing'
		  AND processed_events.lease_until < NOW()
		RETURNING state
	`, eventID, handlerName).Scan(&state)
	if err == nil {
		return consumer.ClaimAcquired, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return consumer.ClaimBusy, err
	}
	if err := s.pool.QueryRow(ctx, `
		SELECT state FROM processed_events
		WHERE event_id = $1 AND handler_name = $2
	`, eventID, handlerName).Scan(&state); err != nil {
		return consumer.ClaimBusy, err
	}
	if state == "processed" {
		return consumer.ClaimProcessed, nil
	}
	return consumer.ClaimBusy, nil
}

func (s *Store) Complete(ctx context.Context, eventID, handlerName string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE processed_events
		SET state = 'processed', processed_at = NOW(), lease_until = NULL
		WHERE event_id = $1 AND handler_name = $2 AND state = 'processing'
	`, eventID, handlerName)
	return err
}

func (s *Store) Release(ctx context.Context, eventID, handlerName string) error {
	_, err := s.pool.Exec(ctx, `
		DELETE FROM processed_events
		WHERE event_id = $1 AND handler_name = $2 AND state = 'processing'
	`, eventID, handlerName)
	return err
}
