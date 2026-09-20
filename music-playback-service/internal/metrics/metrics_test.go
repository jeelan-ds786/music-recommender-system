package metrics

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	playbackevent "github.com/jeelan-ds786/music-recommender-system/music-playback-service/internal/event"
)

func TestMetricsExposeBoundedIngestAndHTTPLabels(t *testing.T) {
	serviceMetrics := New(nil)
	serviceMetrics.ObserveHTTPRequest("POST", "/v1/playback/events", "202", 25*time.Millisecond)
	serviceMetrics.ObserveIngest("accepted", 2)
	serviceMetrics.ObserveIngest("duplicate", 1)
	serviceMetrics.ObserveIngest("rejected", 1)
	serviceMetrics.ObserveBatchSize(3)
	serviceMetrics.ObserveIngestLatency(10 * time.Millisecond)

	request := httptest.NewRequest("GET", "/metrics", nil)
	response := httptest.NewRecorder()
	serviceMetrics.Handler().ServeHTTP(response, request)
	body := response.Body.String()

	for _, expected := range []string{
		`playback_http_requests_total{method="POST",route="/v1/playback/events",status="202"} 1`,
		`playback_ingest_events_total{status="accepted"} 2`,
		`playback_ingest_events_total{status="duplicate"} 1`,
		`playback_ingest_events_total{status="rejected"} 1`,
		`playback_ingest_batch_size_events_count 1`,
		`playback_ingest_duration_seconds_count 1`,
	} {
		if !strings.Contains(body, expected) {
			t.Errorf("metrics output does not contain %q", expected)
		}
	}
	for _, forbidden := range []string{"user_id=", "event_id=", "session_id="} {
		if strings.Contains(body, forbidden) {
			t.Errorf("metrics output contains unbounded label %q", forbidden)
		}
	}
}

func TestInstrumentPublisherCountsFailure(t *testing.T) {
	serviceMetrics := New(nil)
	publisher := serviceMetrics.InstrumentPublisher(&stubPublisher{err: errors.New("kafka unavailable")})

	if err := publisher.Publish(context.Background(), playbackevent.Message{}); err == nil {
		t.Fatal("Publish() error = nil, want failure")
	}

	request := httptest.NewRequest("GET", "/metrics", nil)
	response := httptest.NewRecorder()
	serviceMetrics.Handler().ServeHTTP(response, request)
	body := response.Body.String()
	if !strings.Contains(body, "playback_kafka_publish_failures_total 1") {
		t.Error("publish failure counter was not incremented")
	}
	if !strings.Contains(body, "playback_kafka_publishes_in_flight 0") {
		t.Error("in-flight gauge was not decremented")
	}
}

type stubPublisher struct {
	err error
}

func (p *stubPublisher) Publish(context.Context, playbackevent.Message) error {
	return p.err
}

func (p *stubPublisher) Close() error {
	return nil
}
