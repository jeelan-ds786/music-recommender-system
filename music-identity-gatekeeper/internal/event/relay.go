package event

import (
	"context"
	"time"

	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/logger"
)

type RelayOption func(*Relay)

func WithPollInterval(d time.Duration) RelayOption {
	return func(r *Relay) {
		r.pollInterval = d
	}
}

func WithBatchSize(s int) RelayOption {
	return func(r *Relay) {
		r.batchSize = s
	}
}

func WithMaxAttempts(a int) RelayOption {
	return func(r *Relay) {
		r.maxAttempts = a
	}
}

type Relay struct {
	outbox       *Outbox
	publisher    Publisher
	log          *logger.Logger
	pollInterval time.Duration
	batchSize    int
	maxAttempts  int
	stopCh       chan struct{}
	doneCh       chan struct{}
}

func NewRelay(outbox *Outbox, publisher Publisher, log *logger.Logger, opts ...RelayOption) *Relay {
	r := &Relay{
		outbox:       outbox,
		publisher:    publisher,
		log:          log,
		pollInterval: 1 * time.Second,
		batchSize:    50,
		maxAttempts:  5,
		stopCh:       make(chan struct{}),
		doneCh:       make(chan struct{}),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

func (r *Relay) Start() {
	go r.loop()
}

func (r *Relay) Shutdown(ctx context.Context) error {
	close(r.stopCh)
	select {
	case <-r.doneCh:
		return r.publisher.Close()
	case <-ctx.Done():
		_ = r.publisher.Close()
		return ctx.Err()
	}
}

func (r *Relay) loop() {
	defer close(r.doneCh)

	for {
		select {
		case <-r.stopCh:
			return
		default:
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		events, err := r.outbox.FetchPending(ctx, r.batchSize)
		if err != nil {
			r.log.Error("", "Relay: failed to fetch pending events: %v", err)
			cancel()
			time.Sleep(r.pollInterval)
			continue
		}

		if len(events) == 0 {
			cancel()
			select {
			case <-r.stopCh:
				return
			case <-time.After(r.pollInterval):
				continue
			}
		}

		for _, e := range events {
			select {
			case <-r.stopCh:
				cancel()
				return
			default:
			}

			err := r.publisher.Publish(ctx, e.Topic, e.Key, e.Payload)
			if err != nil {
				r.log.Error("", "Relay: failed to publish event id=%d: %v", e.ID, err)

				if e.Attempts+1 >= r.maxAttempts {
					_ = r.outbox.MarkFailed(ctx, e.ID)
					r.log.Error("", "Relay: marked event id=%d as failed (max attempts reached)", e.ID)
				} else {
					_ = r.outbox.IncrementAttempts(ctx, e.ID)
				}
				continue
			}

			if err := r.outbox.MarkPublished(ctx, e.ID); err != nil {
				r.log.Error("", "Relay: failed to mark event id=%d as published: %v", e.ID, err)
			} else {
				r.log.Debug("", "Relay: successfully published event id=%d", e.ID)
			}
		}
		cancel()
	}
}
