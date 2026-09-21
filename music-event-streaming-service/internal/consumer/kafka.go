package consumer

import (
	"context"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
)

type EventIDDecoder interface {
	EventID(topic string, payload []byte) (string, error)
}

type KafkaReader struct {
	reader  *kafka.Reader
	decoder EventIDDecoder
}

func NewKafkaReader(brokers, topics []string, groupID string, decoder EventIDDecoder) *KafkaReader {
	return &KafkaReader{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:        brokers,
			GroupID:        groupID,
			GroupTopics:    topics,
			MinBytes:       1,
			MaxBytes:       10 << 20,
			MaxWait:        time.Second,
			CommitInterval: 0,
		}),
		decoder: decoder,
	}
}

func (r *KafkaReader) Fetch(ctx context.Context) (Message, error) {
	message, err := r.reader.FetchMessage(ctx)
	if err != nil {
		return Message{}, err
	}
	eventID, err := r.decoder.EventID(message.Topic, message.Value)
	if err != nil {
		return Message{}, err
	}
	headers := make(map[string]string, len(message.Headers))
	for _, header := range message.Headers {
		headers[header.Key] = string(header.Value)
	}
	return Message{
		EventID:   eventID,
		Topic:     message.Topic,
		Partition: message.Partition,
		Offset:    message.Offset,
		Key:       message.Key,
		Value:     message.Value,
		Headers:   headers,
	}, nil
}

func (r *KafkaReader) Commit(ctx context.Context, message Message) error {
	return r.reader.CommitMessages(ctx, kafka.Message{
		Topic:     message.Topic,
		Partition: message.Partition,
		Offset:    message.Offset,
	})
}

func (r *KafkaReader) Close() error {
	return r.reader.Close()
}

type KafkaPinger struct {
	brokers []string
}

func NewKafkaPinger(brokers []string) *KafkaPinger {
	return &KafkaPinger{brokers: brokers}
}

func (p *KafkaPinger) Ping(ctx context.Context) error {
	if len(p.brokers) == 0 {
		return fmt.Errorf("no Kafka brokers configured")
	}
	connection, err := kafka.DialContext(ctx, "tcp", p.brokers[0])
	if err != nil {
		return err
	}
	defer func() { _ = connection.Close() }()
	_, err = connection.ApiVersions()
	return err
}
