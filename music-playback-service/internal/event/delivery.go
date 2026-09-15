package event

import (
	"context"
	"time"
)

type Direct struct {
	outbox    Outbox
	publisher Publisher
}

func NewDirect(outbox Outbox, publisher Publisher) *Direct {
	return &Direct{outbox: outbox, publisher: publisher}
}

func (d *Direct) Publish(ctx context.Context, message Message) {
	if err := d.publisher.Publish(ctx, message); err != nil {
		_ = d.outbox.RecordFailure(ctx, message.ID, err.Error())
		return
	}
	_ = d.outbox.MarkPublished(ctx, message.ID)
}

type Relay struct {
	outbox      Outbox
	publisher   Publisher
	batchSize   int
	maxAttempts int
}

func NewRelay(outbox Outbox, publisher Publisher, batchSize, maxAttempts int) *Relay {
	return &Relay{
		outbox:      outbox,
		publisher:   publisher,
		batchSize:   batchSize,
		maxAttempts: maxAttempts,
	}
}

func (r *Relay) RunOnce(ctx context.Context) error {
	messages, err := r.outbox.FetchPending(ctx, r.batchSize, r.maxAttempts)
	if err != nil {
		return err
	}
	for _, message := range messages {
		if err := r.publisher.Publish(ctx, message); err != nil {
			if recordErr := r.outbox.RecordFailure(ctx, message.ID, err.Error()); recordErr != nil {
				return recordErr
			}
			continue
		}
		if err := r.outbox.MarkPublished(ctx, message.ID); err != nil {
			return err
		}
	}
	return nil
}

func (r *Relay) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = r.RunOnce(ctx)
		}
	}
}
