// Package event holds the Kafka transport for this service's domain
// events: the Publisher contract, a real Kafka-backed implementation, and
// test doubles. Building an event (the envelope + marshaled protobuf
// payload) and calling Publish at the right mutation call sites is E1-SS-09's
// job, not this package's — this package only moves already-encoded bytes
// to a topic.
package event

import (
	"context"
	"fmt"
	"sync"

	"github.com/segmentio/kafka-go"

	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/logger"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/reqid"
)

// Publisher sends an already-encoded event payload to a topic.
type Publisher interface {
	Publish(ctx context.Context, topic, key string, value []byte) error
	Close() error
}

// KafkaPublisher publishes to a real Kafka broker via segmentio/kafka-go.
// One Writer instance serves every topic: leaving Writer.Topic empty and
// setting kafka.Message.Topic per call is kafka-go's supported way to
// share a single writer across topics instead of needing one per topic.
type KafkaPublisher struct {
	writer  *kafka.Writer
	log     *logger.Logger
	brokers []string
}

func NewKafkaPublisher(brokers []string, log *logger.Logger) *KafkaPublisher {
	return &KafkaPublisher{
		writer: &kafka.Writer{
			Addr:     kafka.TCP(brokers...),
			Balancer: &kafka.LeastBytes{},
		},
		log:     log,
		brokers: brokers,
	}
}

// Ping performs one real round-trip to a broker (dial + ApiVersions) to
// confirm connectivity. Used only for a startup log line — a failure here
// does not (and should not) prevent the server from starting, since the
// outbox/relay design exists specifically to tolerate Kafka being
// temporarily unreachable.
func (p *KafkaPublisher) Ping(ctx context.Context) error {
	if len(p.brokers) == 0 {
		return fmt.Errorf("no brokers configured")
	}

	conn, err := kafka.DialContext(ctx, "tcp", p.brokers[0])
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			rid, _ := reqid.FromContext(ctx)
			p.log.Error(rid, "failed to close Kafka connection after ping: %v", closeErr)
		}
	}()

	if _, err := conn.ApiVersions(); err != nil {
		return err
	}

	return nil
}

func (p *KafkaPublisher) Publish(
	ctx context.Context,
	topic string,
	key string,
	value []byte,
) error {

	rid, _ := reqid.FromContext(ctx)

	p.log.Debug(rid, "Starting Publish for topic=%s key=%s", topic, key)

	err := p.writer.WriteMessages(ctx, kafka.Message{
		Topic: topic,
		Key:   []byte(key),
		Value: value,
	})
	if err != nil {
		p.log.Error(rid, "Ending Publish for topic=%s key=%s (failed: %v)", topic, key, err)
		return err
	}

	p.log.Info(rid, "Ending Publish for topic=%s key=%s (published)", topic, key)

	return nil
}

func (p *KafkaPublisher) Close() error {
	return p.writer.Close()
}

// NoopPublisher discards every event. Safe default for contexts where no
// Kafka broker is configured.
type NoopPublisher struct{}

func (NoopPublisher) Publish(context.Context, string, string, []byte) error { return nil }
func (NoopPublisher) Close() error                                          { return nil }

// PublishedMessage is one call recorded by FakePublisher.
type PublishedMessage struct {
	Topic string
	Key   string
	Value []byte
}

// FakePublisher is an in-memory Publisher for tests: it records every
// call instead of talking to Kafka. Exported (not test-only) so other
// packages' tests (e.g. the mutation handlers E1-SS-09 adds) can assert on
// what would have been published, or force a failure via Err.
type FakePublisher struct {
	mu        sync.Mutex
	Published []PublishedMessage
	Err       error
}

func (f *FakePublisher) Publish(_ context.Context, topic, key string, value []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return f.Err
	}

	f.Published = append(f.Published, PublishedMessage{Topic: topic, Key: key, Value: value})

	return nil
}

func (f *FakePublisher) Close() error {
	return nil
}
