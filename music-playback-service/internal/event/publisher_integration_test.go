package event

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
)

func TestKafkaPublisherDeliversPlaybackEvent(t *testing.T) {
	brokerValue := os.Getenv("KAFKA_BROKERS")
	if brokerValue == "" {
		t.Skip("KAFKA_BROKERS not set, skipping Kafka integration test")
	}
	brokers := strings.Split(brokerValue, ",")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	eventType := "play"
	payload := []byte("playback-protobuf-payload")
	key := uuid.NewString()
	metadata, err := kafka.DialContext(ctx, "tcp", brokers[0])
	if err != nil {
		t.Fatalf("dial Kafka metadata: %v", err)
	}
	partitions, err := metadata.ReadPartitions(PlaybackTopic)
	_ = metadata.Close()
	if err != nil {
		t.Fatalf("read partitions: %v", err)
	}
	partitionIDs := make([]int, 0, len(partitions))
	for _, partition := range partitions {
		partitionIDs = append(partitionIDs, partition.ID)
	}
	partitionID := (&kafka.Hash{}).Balance(kafka.Message{Key: []byte(key)}, partitionIDs...)

	reader, err := kafka.DialLeader(ctx, "tcp", brokers[0], PlaybackTopic, partitionID)
	if err != nil {
		t.Fatalf("dial partition leader: %v", err)
	}
	t.Cleanup(func() { _ = reader.Close() })
	nextOffset, err := reader.ReadLastOffset()
	if err != nil {
		t.Fatalf("read end offset: %v", err)
	}
	if _, err := reader.Seek(nextOffset, 0); err != nil {
		t.Fatalf("seek end offset: %v", err)
	}
	if err := reader.SetReadDeadline(time.Now().Add(20 * time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}

	publisher := NewKafkaPublisher(brokers)
	t.Cleanup(func() { _ = publisher.Close() })
	if err := publisher.Publish(ctx, Message{
		Topic:   PlaybackTopic,
		Key:     key,
		Payload: payload,
		Headers: PlaybackHeaders(eventType),
	}); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	message, err := reader.ReadMessage(10e6)
	if err != nil {
		t.Fatalf("ReadMessage() error = %v", err)
	}
	if message.Topic != PlaybackTopic || string(message.Key) != key || string(message.Value) != string(payload) {
		t.Fatalf("received topic=%q key=%q payload=%q", message.Topic, message.Key, message.Value)
	}
	headers := make(map[string]string, len(message.Headers))
	for _, header := range message.Headers {
		headers[header.Key] = string(header.Value)
	}
	wantHeaders := PlaybackHeaders(eventType)
	for name, value := range wantHeaders {
		if headers[name] != value {
			t.Fatalf("header %q = %q, want %q", name, headers[name], value)
		}
	}
}
