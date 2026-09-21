package consumer

import (
	"context"
	"net"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
)

func TestKafkaReaderCommitResumesAfterCommittedOffset(t *testing.T) {
	broker := os.Getenv("KAFKA_BROKERS")
	if broker == "" {
		t.Skip("KAFKA_BROKERS is not set")
	}
	groupID := "streaming-integration-" + uuid.NewString()
	topic := "streaming.integration." + uuid.NewString()
	createTopic(t, broker, topic)
	firstID := uuid.NewString()
	secondID := uuid.NewString()
	publishEvents(t, broker, topic, firstID, secondID)

	firstReader := NewKafkaReader([]string{broker}, []string{topic}, groupID, rawDecoder{})
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	first, err := firstReader.Fetch(ctx)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	if first.EventID != firstID {
		cancel()
		t.Fatalf("first event ID = %s, want %s", first.EventID, firstID)
	}
	if err := firstReader.Commit(ctx, first); err != nil {
		cancel()
		t.Fatal(err)
	}
	cancel()
	if err := firstReader.Close(); err != nil {
		t.Fatal(err)
	}

	secondReader := NewKafkaReader([]string{broker}, []string{topic}, groupID, rawDecoder{})
	t.Cleanup(func() {
		if err := secondReader.Close(); err != nil {
			t.Errorf("close second reader: %v", err)
		}
	})
	ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	second, err := secondReader.Fetch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if second.EventID != secondID {
		t.Fatalf("event after restart = %s, want %s", second.EventID, secondID)
	}
}

type rawDecoder struct{}

func (rawDecoder) EventID(_ string, payload []byte) (string, error) {
	return string(payload), nil
}

func createTopic(t *testing.T, broker, topic string) {
	t.Helper()
	connection, err := kafka.Dial("tcp", broker)
	if err != nil {
		t.Fatal(err)
	}
	controller, err := connection.Controller()
	_ = connection.Close()
	if err != nil {
		t.Fatal(err)
	}
	controllerConnection, err := kafka.Dial("tcp", net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := controllerConnection.Close(); err != nil {
			t.Errorf("close controller connection: %v", err)
		}
	})
	if err := controllerConnection.CreateTopics(kafka.TopicConfig{Topic: topic, NumPartitions: 1, ReplicationFactor: 1}); err != nil {
		t.Fatal(err)
	}
}

func publishEvents(t *testing.T, broker, topic string, eventIDs ...string) {
	t.Helper()
	writer := &kafka.Writer{Addr: kafka.TCP(broker), Balancer: &kafka.Hash{}}
	t.Cleanup(func() {
		if err := writer.Close(); err != nil {
			t.Errorf("close writer: %v", err)
		}
	})
	messages := make([]kafka.Message, 0, len(eventIDs))
	key := []byte(uuid.NewString())
	for _, eventID := range eventIDs {
		messages = append(messages, kafka.Message{Topic: topic, Key: key, Value: []byte(eventID)})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := writer.WriteMessages(ctx, messages...); err != nil {
		t.Fatal(err)
	}
}
