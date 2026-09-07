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

Run package checks from this directory:

```sh
go vet ./...
go test -race ./...
```

Migrations live in `migrations/` and are applied by the one-shot `playback-migrate` Compose service.