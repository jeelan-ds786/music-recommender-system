package event

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/event/eventpb"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/logger"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/reqid"
)

type Emitter struct {
	outbox    *Outbox
	log       *logger.Logger
	publisher Publisher
}

func NewEmitter(outbox *Outbox, log *logger.Logger, publisher Publisher) *Emitter {
	return &Emitter{
		outbox:    outbox,
		log:       log,
		publisher: publisher,
	}
}

// enqueueAndPublish durably enqueues the event, then attempts exactly one
// synchronous publish to Kafka. A publish failure is not returned as an
// error to the caller — the event is already safely durable in the
// outbox, and this thread does not retry; the Relay's background sweep
// (independent of this request, on its own schedule) is solely
// responsible for retrying anything left pending.
func (e *Emitter) enqueueAndPublish(ctx context.Context, tx pgx.Tx, topic, key string, payload []byte) error {
	rid, _ := reqid.FromContext(ctx)

	e.log.Debug(rid, "formed payload for topic=%s key=%s size_bytes=%d", topic, key, len(payload))

	id, err := e.outbox.Enqueue(ctx, tx, topic, key, payload)
	if err != nil {
		return err
	}

	if err := e.publisher.Publish(ctx, topic, key, payload); err != nil {
		e.log.Error(rid, "direct publish to topic=%s key=%s failed, leaving outbox id=%d pending for relay fallback: %v", topic, key, id, err)
		if incErr := e.outbox.IncrementAttempts(ctx, id); incErr != nil {
			e.log.Error(rid, "failed to increment attempts for outbox id=%d: %v", id, incErr)
		}
		return nil
	}

	if err := e.outbox.MarkPublished(ctx, id); err != nil {
		e.log.Error(rid, "published topic=%s key=%s but failed to mark outbox id=%d published: %v", topic, key, id, err)
		return nil
	}

	e.log.Info(rid, "topic=%s key=%s published directly (outbox id=%d)", topic, key, id)

	return nil
}

func (e *Emitter) EmitUserRegistered(ctx context.Context, tx pgx.Tx, userID, email string) error {
	rid, _ := reqid.FromContext(ctx)

	e.log.Debug(rid, "Starting EmitUserRegistered for user_id=%s", userID)

	msg := &eventpb.UserRegistered{
		Metadata: &eventpb.EventMetadata{
			EventId:       uuid.New().String(),
			EventType:     TopicUserRegistered,
			SchemaVersion: 1,
			OccurredAt:    timestamppb.New(time.Now().UTC()),
			UserId:        userID,
		},
		Email: email,
	}

	payload, err := protojson.Marshal(msg)
	if err != nil {
		e.log.Error(rid, "Ending EmitUserRegistered for user_id=%s (failed to marshal: %v)", userID, err)
		return err
	}

	if err := e.enqueueAndPublish(ctx, tx, TopicUserRegistered, userID, payload); err != nil {
		e.log.Error(rid, "Ending EmitUserRegistered for user_id=%s (enqueue failed: %v)", userID, err)
		return err
	}

	e.log.Debug(rid, "Ending EmitUserRegistered for user_id=%s", userID)
	return nil
}

func (e *Emitter) EmitUserPreferenceUpdated(ctx context.Context, tx pgx.Tx, userID string) error {
	rid, _ := reqid.FromContext(ctx)

	e.log.Debug(rid, "Starting EmitUserPreferenceUpdated for user_id=%s", userID)

	msg := &eventpb.UserPreferenceUpdated{
		Metadata: &eventpb.EventMetadata{
			EventId:       uuid.New().String(),
			EventType:     TopicUserPreferenceUpdated,
			SchemaVersion: 1,
			OccurredAt:    timestamppb.New(time.Now().UTC()),
			UserId:        userID,
		},
	}

	payload, err := protojson.Marshal(msg)
	if err != nil {
		e.log.Error(rid, "Ending EmitUserPreferenceUpdated for user_id=%s (failed to marshal: %v)", userID, err)
		return err
	}

	if err := e.enqueueAndPublish(ctx, tx, TopicUserPreferenceUpdated, userID, payload); err != nil {
		e.log.Error(rid, "Ending EmitUserPreferenceUpdated for user_id=%s (enqueue failed: %v)", userID, err)
		return err
	}

	e.log.Debug(rid, "Ending EmitUserPreferenceUpdated for user_id=%s", userID)
	return nil
}
