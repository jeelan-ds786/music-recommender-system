# Music Event Streaming Service

Kafka consumer runtime for playback telemetry and identity interaction events. The service provides at-least-once processing with explicit post-handler offset commits, PostgreSQL idempotency, bounded worker concurrency, dependency readiness, and graceful shutdown.

## Delivery semantics

Each worker fetches one Kafka message, validates its versioned Protobuf contract, atomically claims `(event_id, handler_name)`, invokes the handler, marks the claim processed, and only then commits the Kafka offset. A completed duplicate skips the handler and commits safely. A concurrent duplicate waits for the active claim; failures release the claim and leave the offset uncommitted.

On SIGTERM, workers stop fetching new messages and drain in-flight work for `DRAIN_TIMEOUT`. Successful work is committed before exit. Work still running at the deadline is canceled and remains uncommitted for redelivery.

## Topic contracts

All topics are created explicitly by `kafka-init`; broker auto-create is not required.

| Topic | Partitions | Retention | Key | Producer | Intended consumers |
| --- | ---: | ---: | --- | --- | --- |
| `user.registered` | 1 | 7 days | `user_id` | Identity gatekeeper | Streaming platform |
| `user.preference.updated` | 1 | 7 days | `user_id` | Identity gatekeeper | Streaming platform |
| `user.playlist.updated` | 1 | 7 days | `user_id` | Identity gatekeeper | Streaming platform |
| `playback.event.v1` | 3 | 7 days | `user_id` | Playback service | Streaming platform |
| `recommendation.feedback.v1` | 3 | 7 days | `user_id` | Future recommendation API | Streaming platform |
| `event.retry.v1` | 3 | 7 days | Original key | Streaming service | Retry workers |
| `event.dlq.v1` | 1 | 30 days | Original key | Streaming service | Operators and replay tooling |

Source contracts live under `proto/`; generated Go bindings are checked in under `internal/contracts/`. Existing identity and playback wire formats are preserved. Contract evolution is additive: do not renumber or reuse fields, and increment `schema_version` only with a corresponding consumer rollout. Version 1 is currently accepted.

Regenerate bindings with:

```sh
make proto-gen
```

## Configuration

| Variable | Default | Purpose |
| --- | --- | --- |
| `PORT` | `8083` | Health HTTP port. |
| `DB_URL` | required | PostgreSQL URL for `muse_streaming`. |
| `KAFKA_BROKERS` | required | Comma-separated Kafka brokers. |
| `KAFKA_TOPICS` | `playback.event.v1` | Comma-separated allowlisted source topics. |
| `KAFKA_GROUP_ID` | `music-event-streaming-v1` | Stable live consumer group. |
| `CONSUMER_CONCURRENCY` | `4` | Worker count, bounded from 1 to 32. |
| `DRAIN_TIMEOUT` | `30s` | Maximum graceful in-flight drain time. |

`GET /health/live` is process-only. `GET /health/ready` returns `503` when PostgreSQL or Kafka is unavailable.

## Local verification

From the repository root:

```sh
docker compose up --build streaming-svc
curl -fsS http://localhost:8083/health/ready
```

From this module:

```sh
make check
```

With Docker infrastructure running, apply migrations and run the real idempotency and Kafka offset-resume tests:

```sh
DB_URL='postgres://muse:secret@localhost:5435/muse_streaming?sslmode=disable' \
KAFKA_BROKERS='localhost:9094' \
go test -count=1 ./internal/idempotency ./internal/consumer
```

Retry publication, dead-letter routing, replay operations, and platform metrics are delivered by E4-PR-02 and E4-PR-03.