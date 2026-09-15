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
- `POST /v1/playback/events` accepts one telemetry event; `POST /v1/playback/events:batch` accepts up to 100. Both require `Authorization: Bearer <access token>` issued by `music-identity-gatekeeper` — `user_id` always comes from the token, never the request body. A duplicate `client_event_id` for the same user is an idempotent `202`, not an error. Full request/response shape and error codes: see `internal/playback/dto.go` and `internal/playback/validator.go` (a proper OpenAPI doc is E3-SS-05's job).

Each accepted event and its binary Protobuf outbox envelope commit in one PostgreSQL transaction. After commit, the service publishes to `playback.event.v1` with `user_id` as the Kafka key. Failed direct attempts remain pending for the bounded relay (five attempts); duplicate client events create no additional outbox message. `KAFKA_RELAY_ENABLED=false` disables only fallback polling, not direct publishing.

Kafka configuration:

| Variable | Default | Purpose |
| --- | --- | --- |
| `KAFKA_BROKERS` | unset | Comma-separated brokers. When unset, events remain pending in the outbox. |
| `KAFKA_RELAY_ENABLED` | `true` | Enables or disables fallback polling independently of direct publishing. |
| `KAFKA_RELAY_INTERVAL` | `1s` | Interval between bounded relay sweeps. |

Regenerate the checked-in Protobuf binding with `make proto-gen`. The source contract is `proto/playback/v1/events.proto`.

Run package checks from this directory:

```sh
go vet ./...
go test -race ./...
```

Set `KAFKA_BROKERS=localhost:9094` to include the real-broker delivery integration test; otherwise that test skips while fake-based unit tests still cover direct and relay behavior.

Migrations live in `migrations/` and are applied by the one-shot `playback-migrate` Compose service.