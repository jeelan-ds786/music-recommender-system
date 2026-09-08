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

Run package checks from this directory:

```sh
go vet ./...
go test -race ./...
```

Migrations live in `migrations/` and are applied by the one-shot `playback-migrate` Compose service.