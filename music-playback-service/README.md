# Music Playback Service

Playback telemetry collection service for the music recommender system.

## Local development

Start PostgreSQL, apply migrations, and start the service:

```sh
docker compose up --build playback-svc
```

The service listens on `http://localhost:8082`.

- `GET /health/live` reports process liveness without checking dependencies.
- `GET /health/ready` reports readiness and returns `503` when PostgreSQL is unavailable.
- `GET /metrics` exposes Prometheus metrics.
- `POST /v1/playback/events` accepts one telemetry event.
- `POST /v1/playback/events:batch` accepts up to 100 events.
- `GET /v1/playback/sessions/{sessionID}/events` returns an authenticated user's session events with cursor pagination.

Playback endpoints require `Authorization: Bearer <access token>` issued by `music-identity-gatekeeper`. The `user_id` always comes from the token, never the request body. A duplicate `client_event_id` for the same user is an idempotent `202`, not an error. Invalid bodies exceeding 512 KiB return `413`; headers are limited to 16 KiB.

## Event semantics

The service accepts `play`, `pause`, `seek`, `skip`, `replay`, and `complete` events. `occurred_at` is client event time, while `ingested_at` is assigned by the service. Late and out-of-order events are retained and flagged. Requests more than 24 hours in the future are rejected. Position and duration values must be non-negative, and context is limited to 4 KiB per event.

Each accepted event and its binary Protobuf outbox envelope commit in one PostgreSQL transaction. After commit, the service publishes to `playback.event.v1` with `user_id` as the Kafka key. Failed direct attempts remain pending for the bounded relay (five attempts); duplicate client events create no additional outbox message. `KAFKA_RELAY_ENABLED=false` disables only fallback polling, not direct publishing.

## Configuration

| Variable | Default | Purpose |
| --- | --- | --- |
| `PORT` | `8082` | HTTP listen port. |
| `DB_URL` | required | PostgreSQL connection string for `muse_playback`. |
| `JWT_SECRET` | required | Shared secret used to verify identity access tokens. |
| `LOG_LEVEL` | `info` | One of `debug`, `info`, `error`, or `none`. |
| `KAFKA_BROKERS` | unset | Comma-separated brokers. When unset, events remain pending in the outbox. |
| `KAFKA_RELAY_ENABLED` | `true` | Enables or disables fallback polling independently of direct publishing. |
| `KAFKA_RELAY_INTERVAL` | `1s` | Interval between bounded relay sweeps. |

The HTTP server uses a 5-second header timeout, 15-second read timeout, 30-second write timeout, and 60-second idle timeout.

## Metrics

`GET /metrics` includes:

- `playback_ingest_events_total{status}` for `accepted`, `duplicate`, and `rejected` outcomes.
- `playback_ingest_batch_size_events` and `playback_ingest_duration_seconds` histograms.
- `playback_kafka_outbox_pending_events` and `playback_kafka_outbox_failed_events` gauges.
- `playback_kafka_publish_failures_total` and `playback_kafka_publishes_in_flight`.
- `playback_http_requests_total{method,route,status}` and `playback_http_request_duration_seconds{method,route}`.

Labels are bounded and never include user IDs, event IDs, session IDs, tokens, or raw context. Request completion logs likewise exclude headers and bodies.

## Migrations

Migrations live in `migrations/`. With `DB_URL` configured, apply or roll them back with:

```sh
make migrate-up
make migrate-down
make migrate-down-clean
```

The one-shot `playback-migrate` Compose service applies migrations during local startup.

## Verification

Regenerate the checked-in Protobuf binding with `make proto-gen`. The source contract is `proto/playback/v1/events.proto`.

Run package checks from this directory:

```sh
go vet ./...
go test -race ./...
go build ./cmd/server
```

Set `KAFKA_BROKERS=localhost:9094` to include the real-broker delivery integration test; otherwise that test skips while fake-based unit tests still cover direct and relay behavior.

After starting the service, verify operations with:

```sh
curl -fsS http://localhost:8082/health/ready
curl -fsS http://localhost:8082/metrics | grep '^playback_'
```