package metrics

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/event"
)

type Metrics struct {
	registry      *prometheus.Registry
	httpRequests  *prometheus.CounterVec
	httpLatency   *prometheus.HistogramVec
	kafkaQueue    prometheus.Gauge
	kafkaFailures prometheus.Counter
}

func New(pool *pgxpool.Pool) *Metrics {
	registry := prometheus.NewRegistry()
	metrics := &Metrics{
		registry: registry,
		httpRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "identity",
			Name:      "http_requests_total",
			Help:      "Completed HTTP requests.",
		}, []string{"method", "route", "status"}),
		httpLatency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "identity",
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request latency.",
		}, []string{"method", "route"}),
		kafkaQueue: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "identity",
			Name:      "kafka_publishes_in_flight",
			Help:      "Kafka publishes currently in flight.",
		}),
		kafkaFailures: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "identity",
			Name:      "kafka_publish_failures_total",
			Help:      "Failed Kafka publish attempts.",
		}),
	}

	registry.MustRegister(metrics.httpRequests, metrics.httpLatency, metrics.kafkaQueue, metrics.kafkaFailures)
	if pool != nil {
		registry.MustRegister(newPoolCollector(pool), newOutboxCollector(pool))
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

func (m *Metrics) InstrumentPublisher(publisher event.Publisher) event.Publisher {
	return &instrumentedPublisher{Publisher: publisher, metrics: m}
}

type instrumentedPublisher struct {
	event.Publisher
	metrics *Metrics
}

func (p *instrumentedPublisher) Publish(ctx context.Context, topic, key string, value []byte) error {
	p.metrics.kafkaQueue.Inc()
	defer p.metrics.kafkaQueue.Dec()

	if err := p.Publisher.Publish(ctx, topic, key, value); err != nil {
		p.metrics.kafkaFailures.Inc()
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
		pending: prometheus.NewDesc("identity_kafka_outbox_pending_events", "Events waiting for Kafka publication.", nil, nil),
		failed:  prometheus.NewDesc("identity_kafka_outbox_failed_events", "Events that exhausted Kafka publication retries.", nil, nil),
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
			COUNT(*) FILTER (WHERE status = 'pending'),
			COUNT(*) FILTER (WHERE status = 'failed')
		FROM kafka_integration
	`).Scan(&pending, &failed)
	if err != nil {
		return
	}

	ch <- prometheus.MustNewConstMetric(c.pending, prometheus.GaugeValue, float64(pending))
	ch <- prometheus.MustNewConstMetric(c.failed, prometheus.GaugeValue, float64(failed))
}

type poolCollector struct {
	pool     *pgxpool.Pool
	acquired *prometheus.Desc
	idle     *prometheus.Desc
	max      *prometheus.Desc
}

func newPoolCollector(pool *pgxpool.Pool) *poolCollector {
	return &poolCollector{
		pool:     pool,
		acquired: prometheus.NewDesc("identity_db_pool_acquired_connections", "Acquired PostgreSQL connections.", nil, nil),
		idle:     prometheus.NewDesc("identity_db_pool_idle_connections", "Idle PostgreSQL connections.", nil, nil),
		max:      prometheus.NewDesc("identity_db_pool_max_connections", "Maximum PostgreSQL connections.", nil, nil),
	}
}

func (c *poolCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.acquired
	ch <- c.idle
	ch <- c.max
}

func (c *poolCollector) Collect(ch chan<- prometheus.Metric) {
	stats := c.pool.Stat()
	ch <- prometheus.MustNewConstMetric(c.acquired, prometheus.GaugeValue, float64(stats.AcquiredConns()))
	ch <- prometheus.MustNewConstMetric(c.idle, prometheus.GaugeValue, float64(stats.IdleConns()))
	ch <- prometheus.MustNewConstMetric(c.max, prometheus.GaugeValue, float64(stats.MaxConns()))
}
