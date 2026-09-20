package metrics

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	playbackevent "github.com/jeelan-ds786/music-recommender-system/music-playback-service/internal/event"
)

const outboxMaxAttempts = 5

type Metrics struct {
	registry        *prometheus.Registry
	httpRequests    *prometheus.CounterVec
	httpLatency     *prometheus.HistogramVec
	ingestEvents    *prometheus.CounterVec
	ingestBatchSize prometheus.Histogram
	ingestLatency   prometheus.Histogram
	publishInFlight prometheus.Gauge
	publishFailures prometheus.Counter
}

func New(pool *pgxpool.Pool) *Metrics {
	registry := prometheus.NewRegistry()
	metrics := &Metrics{
		registry: registry,
		httpRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "playback",
			Name:      "http_requests_total",
			Help:      "Completed HTTP requests.",
		}, []string{"method", "route", "status"}),
		httpLatency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "playback",
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request latency.",
		}, []string{"method", "route"}),
		ingestEvents: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "playback",
			Name:      "ingest_events_total",
			Help:      "Playback events by ingestion outcome.",
		}, []string{"status"}),
		ingestBatchSize: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: "playback",
			Name:      "ingest_batch_size_events",
			Help:      "Number of playback events submitted in each batch.",
			Buckets:   []float64{1, 5, 10, 25, 50, 100},
		}),
		ingestLatency: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: "playback",
			Name:      "ingest_duration_seconds",
			Help:      "Playback ingestion service latency.",
		}),
		publishInFlight: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "playback",
			Name:      "kafka_publishes_in_flight",
			Help:      "Kafka publishes currently in flight.",
		}),
		publishFailures: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "playback",
			Name:      "kafka_publish_failures_total",
			Help:      "Failed Kafka publish attempts.",
		}),
	}

	registry.MustRegister(
		metrics.httpRequests,
		metrics.httpLatency,
		metrics.ingestEvents,
		metrics.ingestBatchSize,
		metrics.ingestLatency,
		metrics.publishInFlight,
		metrics.publishFailures,
	)
	if pool != nil {
		registry.MustRegister(newOutboxCollector(pool))
	}
	return metrics
}

func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

func (m *Metrics) ObserveHTTPRequest(method, route, status string, latency time.Duration) {
	m.httpRequests.WithLabelValues(method, route, status).Inc()
	m.httpLatency.WithLabelValues(method, route).Observe(latency.Seconds())
}

func (m *Metrics) ObserveIngest(status string, count int) {
	m.ingestEvents.WithLabelValues(status).Add(float64(count))
}

func (m *Metrics) ObserveBatchSize(size int) {
	m.ingestBatchSize.Observe(float64(size))
}

func (m *Metrics) ObserveIngestLatency(latency time.Duration) {
	m.ingestLatency.Observe(latency.Seconds())
}

func (m *Metrics) InstrumentPublisher(publisher playbackevent.Publisher) playbackevent.Publisher {
	return &instrumentedPublisher{Publisher: publisher, metrics: m}
}

type instrumentedPublisher struct {
	playbackevent.Publisher
	metrics *Metrics
}

func (p *instrumentedPublisher) Publish(ctx context.Context, message playbackevent.Message) error {
	p.metrics.publishInFlight.Inc()
	defer p.metrics.publishInFlight.Dec()

	if err := p.Publisher.Publish(ctx, message); err != nil {
		p.metrics.publishFailures.Inc()
		return err
	}
	return nil
}

type outboxCollector struct {
	pool    *pgxpool.Pool
	pending *prometheus.Desc
	failed  *prometheus.Desc
}

func newOutboxCollector(pool *pgxpool.Pool) *outboxCollector {
	return &outboxCollector{
		pool:    pool,
		pending: prometheus.NewDesc("playback_kafka_outbox_pending_events", "Events waiting for Kafka publication.", nil, nil),
		failed:  prometheus.NewDesc("playback_kafka_outbox_failed_events", "Events that exhausted Kafka publication retries.", nil, nil),
	}
}

func (c *outboxCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.pending
	ch <- c.failed
}

func (c *outboxCollector) Collect(ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	var pending, failed int64
	err := c.pool.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE published_at IS NULL AND attempts < $1),
			COUNT(*) FILTER (WHERE published_at IS NULL AND attempts >= $1)
		FROM outbox_events
	`, outboxMaxAttempts).Scan(&pending, &failed)
	if err != nil {
		return
	}

	ch <- prometheus.MustNewConstMetric(c.pending, prometheus.GaugeValue, float64(pending))
	ch <- prometheus.MustNewConstMetric(c.failed, prometheus.GaugeValue, float64(failed))
}
