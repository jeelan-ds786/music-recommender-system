package consumer

import (
	"context"
	"errors"
	"time"
)

var ErrInvalidMessage = errors.New("invalid message")

type ClaimState int

const (
	ClaimAcquired ClaimState = iota
	ClaimProcessed
	ClaimBusy
)

const claimPollInterval = 50 * time.Millisecond

type Message struct {
	EventID   string
	Topic     string
	Partition int
	Offset    int64
	Key       []byte
	Value     []byte
	Headers   map[string]string
}

type Reader interface {
	Fetch(context.Context) (Message, error)
	Commit(context.Context, Message) error
	Close() error
}

type Handler interface {
	Name() string
	Handle(context.Context, Message) error
}

type IdempotencyStore interface {
	Claim(context.Context, string, string) (ClaimState, error)
	Complete(context.Context, string, string) error
	Release(context.Context, string, string) error
}

type Consumer struct {
	reader  Reader
	handler Handler
	store   IdempotencyStore
}

func New(reader Reader, handler Handler, store IdempotencyStore) *Consumer {
	return &Consumer{reader: reader, handler: handler, store: store}
}

func (c *Consumer) Run(fetchCtx, processCtx context.Context) (runErr error) {
	defer func() {
		if closeErr := c.reader.Close(); runErr == nil && closeErr != nil {
			runErr = closeErr
		}
	}()
	for {
		message, err := c.reader.Fetch(fetchCtx)
		if err != nil {
			if fetchCtx.Err() != nil {
				return nil
			}
			return err
		}
		if err := c.process(processCtx, message); err != nil {
			return err
		}
	}
}

func RunGroup(ctx context.Context, drainTimeout time.Duration, consumers ...*Consumer) error {
	fetchCtx, stopFetching := context.WithCancel(context.Background())
	processCtx, stopProcessing := context.WithCancel(context.Background())
	defer stopFetching()
	defer stopProcessing()

	results := make(chan error, len(consumers))
	for _, worker := range consumers {
		go func() { results <- worker.Run(fetchCtx, processCtx) }()
	}

	remaining := len(consumers)
	var firstErr error
	shutdown := ctx.Done()
	var drain <-chan time.Time
	for remaining > 0 {
		select {
		case err := <-results:
			remaining--
			if err != nil && firstErr == nil {
				firstErr = err
				stopFetching()
				stopProcessing()
			}
		case <-shutdown:
			stopFetching()
			shutdown = nil
			timer := time.NewTimer(drainTimeout)
			defer timer.Stop()
			drain = timer.C
		case <-drain:
			stopProcessing()
			drain = nil
		}
	}
	return firstErr
}

func (c *Consumer) process(ctx context.Context, message Message) error {
	if message.EventID == "" {
		return ErrInvalidMessage
	}

	claim, err := c.waitForClaim(ctx, message.EventID)
	if err != nil {
		return err
	}
	if claim == ClaimProcessed {
		return c.reader.Commit(ctx, message)
	}

	if err := c.handler.Handle(ctx, message); err != nil {
		_ = c.store.Release(context.WithoutCancel(ctx), message.EventID, c.handler.Name())
		return err
	}
	if err := c.store.Complete(ctx, message.EventID, c.handler.Name()); err != nil {
		return err
	}

	return c.reader.Commit(ctx, message)
}

func (c *Consumer) waitForClaim(ctx context.Context, eventID string) (ClaimState, error) {
	for {
		claim, err := c.store.Claim(ctx, eventID, c.handler.Name())
		if err != nil || claim != ClaimBusy {
			return claim, err
		}
		timer := time.NewTimer(claimPollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ClaimBusy, ctx.Err()
		case <-timer.C:
		}
	}
}
