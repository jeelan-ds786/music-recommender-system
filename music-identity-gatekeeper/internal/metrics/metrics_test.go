package metrics

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/event"
)

func TestHTTPMetricsUseTemplatedRouteLabels(t *testing.T) {
	serviceMetrics := New(nil)
	serviceMetrics.ObserveHTTPRequest(http.MethodGet, "/me/likes/songs/{songID}", "200", 20*time.Millisecond)
	response := httptest.NewRecorder()
	serviceMetrics.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := response.Body.String()

	if !strings.Contains(body, `identity_http_requests_total{method="GET",route="/me/likes/songs/{songID}",status="200"} 1`) {
		t.Fatalf("templated request metric missing: %s", body)
	}
	if strings.Contains(body, "user_id") {
		t.Fatalf("metrics contain a user ID label: %s", body)
	}
}

func TestPublisherMetricsRecordFailuresAndClearQueue(t *testing.T) {
	serviceMetrics := New(nil)
	publisher := serviceMetrics.InstrumentPublisher(&event.FakePublisher{Err: errors.New("broker unavailable")})
	if err := publisher.Publish(context.Background(), "topic", "key", []byte("payload")); err == nil {
		t.Fatal("Publish() error = nil")
	}

	response := httptest.NewRecorder()
	serviceMetrics.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := response.Body.String()
	for _, metric := range []string{
		"identity_kafka_publish_failures_total 1",
		"identity_kafka_publishes_in_flight 0",
	} {
		if !strings.Contains(body, metric) {
			t.Fatalf("metric %q missing: %s", metric, body)
		}
	}
}
