package event

import (
	"context"
	"sync"

	"github.com/segmentio/kafka-go"
)

type Publisher interface {
	Publish(context.Context, Message) error
	Close() error
}

type KafkaPublisher struct {
	writer *kafka.Writer
}

func NewKafkaPublisher(brokers []string) *KafkaPublisher {
	return &KafkaPublisher{writer: &kafka.Writer{
		Addr:     kafka.TCP(brokers...),
		Balancer: &kafka.Hash{},
	}}
}

func (p *KafkaPublisher) Publish(ctx context.Context, message Message) error {
	headers := make([]kafka.Header, 0, len(message.Headers))
	for key, value := range message.Headers {
		headers = append(headers, kafka.Header{Key: key, Value: []byte(value)})
	}

	return p.writer.WriteMessages(ctx, kafka.Message{
		Topic:   message.Topic,
		Key:     []byte(message.Key),
		Value:   message.Payload,
		Headers: headers,
	})
}

func (p *KafkaPublisher) Close() error {
	return p.writer.Close()
}

type FakePublisher struct {
	mu        sync.Mutex
	Published []Message
	Err       error
}

func (p *FakePublisher) Publish(_ context.Context, message Message) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.Err != nil {
		return p.Err
	}
	p.Published = append(p.Published, message)
	return nil
}

func (p *FakePublisher) Close() error {
	return nil
}
