# Epic 2 — Four-Day Sprint

**Goal:** Build and release the core Music Catalog Management Service — schema, CRUD, browse, events, and gRPC — in four days with two developers.

**Team:** Developer A and Developer B
**Capacity assumption:** 6-8 focused hours per developer per day, with Day 4 run as a combined integration/release day (see below)
**Sprint length:** 4 consecutive days
**Release target:** `v0.2.0`

---

## Scope note: this is a compressed sprint, not the full epic

This was originally scoped as a seven-day, 13-ticket sprint (schema →
CRUD → browse → bulk import → Kafka → gRPC → observability → docs → E2E).
Compressed to four days, three lower-priority tickets don't fit and are
**deferred to an Epic 2 Sprint 2**, the same way Epic 1 needed a Sprint 2
to finish logout, observability, docs, and playlists after its own
seven-day sprint:

- **E2-SS-07** — Bulk catalog import / admin ingestion pipeline (P1)
- **E2-SS-11** — Full metrics, structured logging, and health depth (P1)
- **E2-SS-12** — Full README/OpenAPI/Postman/architecture-diagram documentation (P1)

These are genuinely deferred, not dropped — their ticket numbers are
reserved and will carry their original scope into Epic 2 Sprint 2 once
that's planned. This sprint ships the P0 core: schema, CRUD, browse,
Kafka events, and gRPC, plus a minimal README stub and a basic health
check so the service is usable and demoable, just not fully observable or
bulk-seedable yet.

---

## Starting point

This is a new, greenfield service — unlike Epic 1, nothing exists in the
repository for it yet. It reuses proven patterns and shared local
infrastructure from Epic 1's `music-identity-gatekeeper` rather than
inventing new ones:

- The same Go / `chi` / `pgx/v5` / `golang-migrate` / `segmentio/kafka-go` /
  `google.golang.org/grpc` stack (`zap` and `prometheus/client_golang`
  arrive with the deferred E2-SS-11 in Sprint 2).
- The same repo-root `docker-compose.yml` — Postgres, Kafka, and
  `kafka-init` are already running for Epic 1. This sprint adds a
  `catalog-svc` service block and a second Postgres database
  (`muse_catalog`), and extends `kafka-init` with one new topic. It does
  not stand up a second broker or duplicate infrastructure.
- The same handler → service → repository layering, `response.JSON` /
  `response.Error` / `response.ValidationError` error-shape convention.
- The same transactional-outbox Kafka publishing design proven in
  E1-SS-09 (enqueue durably, one direct publish attempt, background relay
  as fallback) — reimplemented in this service, not shared as a library.

This sprint builds the catalog domain (artists, albums, songs), its
public read / admin-protected write HTTP API, Kafka event publishing, and
an internal gRPC API that later epics (Feature Store, Candidate
Generation, Search & Discovery) will consume.

**Explicitly out of scope for this sprint** (beyond the three deferred
tickets above): full-text or semantic search (Epic 12), audio feature
extraction / embeddings (Epic 6), popularity scoring from real listening
behavior (Epic 13), and any consumer of the Kafka events this service
publishes.

---

## Recommended ownership

### Developer A — Runtime, authorization, and service-interface owner

- Service scaffolding and admin-key authorization middleware
- Album CRUD and artist-album linking
- Catalog browse/filter/pagination API
- Internal gRPC catalog API
- Release integration on Day 4
- Shared integration files such as `cmd/server/main.go`, the
  `catalog-svc` block in the root `docker-compose.yml`, and final
  dependency reconciliation

### Developer B — Catalog domain and event owner

- Catalog schema (migrations) and domain repositories
- Artist CRUD and full read API
- Song CRUD and catalog metadata
- Kafka catalog event schema, publisher, and event publishing
- Kafka topic additions to the root `docker-compose.yml`

Same shape and shared-file rule as Epic 1 and as this sprint's original
seven-day draft: B exposes constructors and handlers; A wires them into
the server. A is the merge owner for `cmd/server/main.go`, `go.mod`, and
`go.sum`; B owns migrations, event Protobuf definitions, and the
`catalog-svc`/Kafka-topic additions to `docker-compose.yml`.

---

## Definition of Done (for this four-day sprint)

- All HTTP endpoints for artists, albums, songs, and browse/filter are implemented
- Every catalog mutation is protected by the admin key; every read endpoint is public
- Artist/album/song relationships are enforced with real foreign keys, not just documented convention
- `catalog.entity.updated` is published for every successful create, update, and delete
- gRPC `GetArtist`, `GetSong`, and `BatchGetSongs` work on the internal gRPC port
- A basic `/health/live` and `/health/ready` exist (full `/metrics` depth is Sprint 2's E2-SS-11)
- `go test ./...`, `go vet ./...`, Docker build, migrations, and a basic end-to-end smoke test pass
- A minimal README (quick start + env vars) exists so the service is runnable (full OpenAPI/Postman/diagram is Sprint 2's E2-SS-12)
- The release is tagged `v0.2.0` only after the release branch is green

**Not required for this sprint's Definition of Done** (see Sprint 2):
bulk import, full Prometheus metrics, full structured logging, OpenAPI
spec, Postman collection, architecture diagram.

---

# Tickets

## E2-SS-01 — Catalog schema and domain contracts

**Owner:** B
**Priority:** P0
**Estimate:** 6 hours
**Dependencies:** None
**Merge position:** 1

### Work

- Add migration `000001_create_artists`: `id UUID PRIMARY KEY DEFAULT gen_random_uuid()`, `name TEXT NOT NULL`, `bio TEXT`, `image_url TEXT`, `popularity_score INT NOT NULL DEFAULT 0`, `external_id TEXT UNIQUE` (nullable — reserved for Sprint 2's bulk-import idempotency), `created_at`/`updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`.
- Add migration `000002_create_albums`: `id UUID PRIMARY KEY`, `artist_id UUID NOT NULL REFERENCES artists(id) ON DELETE CASCADE`, `title TEXT NOT NULL`, `release_year INT`, `cover_url TEXT`, `album_type TEXT NOT NULL DEFAULT 'album'`, `external_id TEXT UNIQUE`, timestamps. Index on `artist_id`.
- Add migration `000003_create_songs`: `id UUID PRIMARY KEY`, `title TEXT NOT NULL`, `primary_artist_id UUID NOT NULL REFERENCES artists(id)`, `album_id UUID REFERENCES albums(id) ON DELETE SET NULL`, `duration_ms INT NOT NULL`, `genre_tags TEXT[] NOT NULL DEFAULT '{}'`, `mood_tags TEXT[] NOT NULL DEFAULT '{}'`, `release_year INT`, `popularity_score INT NOT NULL DEFAULT 0`, `explicit BOOLEAN NOT NULL DEFAULT false`, `isrc TEXT UNIQUE`, `acoustic_features JSONB NOT NULL DEFAULT '{}'` (reserved for Epic 6), `external_id TEXT UNIQUE`, timestamps. GIN indexes on `genre_tags` and `mood_tags`; btree index on `(popularity_score DESC, id DESC)`; index on `primary_artist_id` and `album_id`.
- Add migration `000004_create_song_featured_artists`: `song_id`/`artist_id` FKs, `PRIMARY KEY (song_id, artist_id)` — a normalized many-to-many table, not an array column (same lesson as Epic 1's E1-SS-05).
- All four migrations reversible.
- Define `internal/artist`, `internal/album`, and `internal/song` domain models, repository interfaces, and service interfaces. No HTTP, Kafka, or gRPC imports.

### Acceptance criteria

- All four migrations apply and roll back cleanly on a fresh `muse_catalog` database.
- A song can be created referencing an existing artist and album; creating one against a nonexistent artist fails at the database level.
- Domain contracts compile with no HTTP/Kafka/gRPC imports.
- `go test ./...` passes.

---

## E2-SS-02 — Service scaffolding and admin authorization

**Owner:** A
**Priority:** P0
**Estimate:** 7 hours
**Dependencies:** None
**Merge position:** 2; may be reviewed in parallel with E2-SS-01

### Work

- Initialize the service (`cmd/server/main.go`, `internal/response`, `internal/db` — copy the shapes proven in `music-identity-gatekeeper`, not a shared import). Basic `log`-package logging is fine for now; `zap` arrives with Sprint 2's E2-SS-11.
- Add a Dockerfile (multi-stage, distroless runtime, matching Epic 1's pattern) and a `catalog-svc` block in the root `docker-compose.yml`, plus a `muse_catalog` database on the existing `postgres` container.
- Implement `ADMIN_API_KEY`-based write authorization: mutations are protected by a static key checked via an `X-Admin-Key` header with a constant-time comparison — not by reusing `music-identity-gatekeeper`'s end-user JWTs, since catalog writes are operator/ETL actions, not end-user actions. All `GET` endpoints stay public.
- Add basic `/health/live` and `/health/ready` (Postgres check, short timeout) — full metrics depth is Sprint 2.
- Table-driven tests: missing/incorrect/well-formed `X-Admin-Key`, health endpoint behavior when Postgres is down.

### Acceptance criteria

- The server boots with only placeholder env vars set and `/health/live` returns `200`.
- A mutation route protected by `AdminKeyMiddleware` returns a structured `401` for a missing or incorrect key, and passes through for a correct one.
- No read route requires the admin key.
- `go vet ./...` and `go test -race ./...` pass.

---

## E2-SS-03 — Artist CRUD and full read API

**Owner:** B
**Priority:** P0
**Estimate:** 6 hours
**Dependencies:** E2-SS-01
**Merge position:** 3

### Work

- Implement `internal/artist`'s repository (Postgres) and service.
- `POST /artists` (admin key) — create; `name` required and non-empty.
- `GET /artists/{id}` — full public read.
- `GET /artists` — paginated public list, sortable by `name` or `popularity_score`.
- `PATCH /artists/{id}` (admin key) — pointer-field patch, same pattern as Epic 1's `PATCH /me`.
- `DELETE /artists/{id}` (admin key) — hard delete; document that it cascades to albums and songs.
- Handler and repository tests.

### Acceptance criteria

- Unauthenticated `GET` succeeds; unauthenticated `POST`/`PATCH`/`DELETE` return structured `401`.
- Invalid UUID → `400`; missing artist → structured `404`.
- Patching one field does not change any omitted field.
- Repository queries are parameterized.

---

## E2-SS-04 — Album CRUD and artist-album linking

**Owner:** A
**Priority:** P0
**Estimate:** 6 hours
**Dependencies:** E2-SS-01, E2-SS-03
**Merge position:** 4

### Work

- Implement `internal/album`'s repository and service.
- `POST /albums` (admin key) — requires `artist_id`; validate it exists at the service layer, return `404 ARTIST_NOT_FOUND` rather than a raw FK error.
- `GET /albums/{id}` — public full read.
- `GET /artists/{id}/albums` — public, paginated, ordered by `release_year DESC`.
- `PATCH /albums/{id}`, `DELETE /albums/{id}` (admin key).
- Validate `release_year` is a plausible four-digit year.

### Acceptance criteria

- Creating an album against a nonexistent artist returns `404 ARTIST_NOT_FOUND`.
- `GET /artists/{id}/albums` paginates with no duplicates or omissions.
- Same auth/validation acceptance shape as E2-SS-03.

---

## E2-SS-05 — Song CRUD and catalog metadata

**Owner:** B
**Priority:** P0
**Estimate:** 7 hours
**Dependencies:** E2-SS-01, E2-SS-03, E2-SS-04
**Merge position:** 5

### Work

- Implement `internal/song`'s repository and service, including `song_featured_artists` join-table operations.
- `POST /songs` (admin key): `title`, `primary_artist_id` (must exist), `album_id` (optional; if present, must belong to `primary_artist_id`), `duration_ms`, `genre_tags`/`mood_tags` (capped at 5, reusing Epic 1's onboarding validator pattern), `release_year`, `explicit`, `isrc` (optional, format-validated), `featured_artist_ids` (optional).
- `GET /songs/{id}` — public full read.
- `PATCH /songs/{id}`, `DELETE /songs/{id}` (admin key).
- `popularity_score` is read-only through this API (defaults to `0`; a later epic owns updating it).

### Acceptance criteria

- A nonexistent `primary_artist_id`, or an `album_id` belonging to a different artist, returns structured `400`/`404`.
- `genre_tags`/`mood_tags` capped at 5.
- A featured artist can't be attached twice (join-table PK enforces it).
- `go test ./...` passes; queries parameterized.

---

## E2-SS-06 — Catalog browse, filter, and pagination API

**Owner:** A
**Priority:** P0
**Estimate:** 6 hours
**Dependencies:** E2-SS-05
**Merge position:** 6

### Work

- `GET /catalog/songs?genre=&mood=&artist_id=&year=&sort=popularity|release_year&cursor=&limit=` — public.
- Cursor-based pagination, same opaque-cursor approach as Epic 1's `GET /me/likes/songs`.
- Combine filters with AND semantics; validate `sort` against an allowlist.
- Explicitly out of scope: free-text/semantic search (Epic 12's job).

### Acceptance criteria

- Pagination has no duplicates/omissions across a stable seeded dataset, with and without filters.
- Invalid `sort` or malformed `cursor` → `400`.
- Combined filters narrow results correctly.
- No endpoint here requires the admin key.

---

## E2-SS-07 — Bulk catalog import / admin ingestion pipeline — **deferred to Epic 2 Sprint 2**

**Priority:** P1 (deferred, not dropped)
**Original estimate:** 8 hours
**Reason for deferral:** doesn't fit four days once schema/CRUD/browse/Kafka/gRPC are prioritized; catalog rows can be seeded through `POST /artists`/`/albums`/`/songs` for this sprint's smoke test.

See the original Epic 2 seven-day draft for full scope (CLI + HTTP import, `external_id` upsert idempotency, partial-batch failure reporting). Re-scope when Sprint 2 is planned.

---

## E2-SS-08 — Kafka catalog event contract and local infrastructure

**Owner:** B
**Priority:** P0
**Estimate:** 6 hours
**Dependencies:** E2-SS-01
**Merge position:** 7; contract can be prepared earlier, but merge after stable domain models

### Work

- Define a `CatalogEntityUpdated` Protobuf message: the standard `EventMetadata` envelope plus `entity_type` (`ARTIST`/`ALBUM`/`SONG`), `entity_id`, and `operation` (`CREATED`/`UPDATED`/`DELETED`).
- Add `segmentio/kafka-go` behind a small `Publisher` interface, same shape as `music-identity-gatekeeper`'s.
- Extend the existing `kafka-init` job to also create the `catalog.entity.updated` topic (`--if-not-exists`).
- Provide a no-op/fake publisher for unit tests.

### Acceptance criteria

- Protobuf generation is reproducible via a Make target.
- The service connects to the existing local Kafka broker after `kafka-init`'s health check passes.
- Unit tests don't require Kafka.
- The event carries enough metadata for downstream deduplication and schema evolution.

---

## E2-SS-09 — Publish catalog mutation events

**Owner:** B
**Priority:** P0
**Estimate:** 7 hours
**Dependencies:** E2-SS-05, E2-SS-08
**Merge position:** 8

### Work

- Publish `catalog.entity.updated` after every successful create/update/delete on artists, albums, and songs.
- Reuse the outbox-then-direct-publish-with-fallback-relay design proven in E1-SS-09 — build it this way from the start instead of discovering the fire-and-forget gap the way Epic 1 did.
- Keep event creation separate from Kafka transport.

### Acceptance criteria

- Every successful mutation emits exactly the documented event with the correct `entity_type`/`operation`.
- Failed database mutations emit no event.
- Disabling the background relay does not disable the direct-publish path — verify explicitly.
- Unit tests assert payloads via a fake publisher; an integration test proves delivery to the local topic.

---

## E2-SS-10 — Internal gRPC catalog API

**Owner:** A
**Priority:** P0
**Estimate:** 7 hours
**Dependencies:** E2-SS-05, E2-SS-06
**Merge position:** 9

### Work

- Define `CatalogService` in `proto/catalog.proto`: `GetArtist`, `GetSong`, `BatchGetSongs` (by ID list — the method downstream services like Candidate Generation and the Feature Store will actually call at scale).
- Generate Go client/server code reproducibly.
- Run gRPC on a dedicated internal port (`50052`; `50051` belongs to `music-identity-gatekeeper`), restricted to the internal Docker network.
- Graceful shutdown alongside HTTP.

### Acceptance criteria

- `BatchGetSongs` with a mix of valid/invalid/missing IDs returns valid ones plus a per-ID not-found list.
- Invalid UUID → `InvalidArgument`; missing single lookup → `NotFound`.
- A test gRPC client verifies all three RPCs.
- HTTP and gRPC servers shut down cleanly together.

---

## E2-SS-11 — Metrics, structured logging, and health — **deferred to Epic 2 Sprint 2**

**Priority:** P1 (deferred, not dropped)
**Original estimate:** 8 hours
**Reason for deferral:** E2-SS-02 already ships a basic health check; full Prometheus/zap depth doesn't fit four days without cutting P0 scope.

See the original Epic 2 seven-day draft for full scope. Re-scope when Sprint 2 is planned.

---

## E2-SS-12 — Documentation and API artifacts — **deferred to Epic 2 Sprint 2**

**Priority:** P1 (deferred, not dropped)
**Original estimate:** 8 hours
**Reason for deferral:** a minimal README (quick start + env vars, written as part of E2-SS-13) covers "a new developer can run it"; full OpenAPI/Postman/architecture-diagram polish is deferred, same as Epic 1 deferred part of its own documentation depth into its Sprint 2.

See the original Epic 2 seven-day draft for full scope. Re-scope when Sprint 2 is planned.

---

## E2-SS-13 — Basic end-to-end test, CI, and release

**Owner:** A as release captain; B pairs on failures
**Priority:** P0
**Estimate:** 6 hours
**Dependencies:** E2-SS-01 through E2-SS-06, E2-SS-08, E2-SS-09, E2-SS-10
**Merge position:** 10, always last

### Work

- Add a basic end-to-end flow: migrate database → create an artist, album, and song via the HTTP API → browse/filter and confirm the new song is found → verify the `catalog.entity.updated` create event reached Kafka → call gRPC `BatchGetSongs` and confirm it's returned → update the song and verify the update event → delete it and verify the delete event and a subsequent `404`.
- Write a minimal README stub: quick start, prerequisites, env vars (including `ADMIN_API_KEY`) — full documentation depth is Sprint 2's E2-SS-12.
- Run tests and race detection in CI; build the Docker image in CI.
- Validate migration up and down against PostgreSQL 16.
- Run `go vet ./...`.
- Create `v0.2.0` after the default branch is green.

### Acceptance criteria

- CI is green from a clean checkout.
- No test relies on a developer's existing local database state.
- Docker Compose reaches healthy status for `postgres`, `kafka`, `kafka-init`, and `catalog-svc`.
- The basic smoke test passes twice consecutively.
- A README exists covering how to start the service locally.
- Tag `v0.2.0` points to the exact green release commit.

---

# Four-Day Timeline

| Day | Developer A | Developer B | Required merges by end of day |
|---|---|---|---|
| **Day 1 — Contracts and scaffolding** | E2-SS-02 service scaffolding and admin authorization | E2-SS-01 catalog schema and domain contracts | Merge **01**, then **02** |
| **Day 2 — Artist and album APIs** | E2-SS-04 album CRUD and artist linking | E2-SS-03 artist CRUD and full read API | Merge **03**, then **04** |
| **Day 3 — Song catalog, browse, and eventing contract** | E2-SS-06 browse, filter, and pagination API | E2-SS-05 song CRUD and metadata; then start E2-SS-08 (Kafka contract) | Merge **05**, then **06** |
| **Day 4 — Integration and release (combined, intensive)** | E2-SS-10 gRPC catalog API; then pair as release captain on E2-SS-13 | Finish E2-SS-08, then E2-SS-09 catalog event publishing; then pair on E2-SS-13 | Merge **08**, then **09**, then **10**, then **13**; tag `v0.2.0` |

Day 4 is intentionally the busiest day and is run as a combined
integration push rather than two independent tickets — the same way Epic
1's own Day 7 had both developers pairing rather than working two
separate full-day tickets. If Day 4 doesn't fully fit, cut E2-SS-13's E2E
scope to the create→browse→event chain only (drop the update/delete legs)
before cutting anything else; do not skip the admin-key or FK-integrity
acceptance criteria from earlier days.

---

# Merge Train

```text
1.  E2-SS-01  Catalog schema + domain contracts
2.  E2-SS-02  Service scaffolding + admin authorization
3.  E2-SS-03  Artist CRUD + full read API
4.  E2-SS-04  Album CRUD + artist linking
5.  E2-SS-05  Song CRUD + catalog metadata
6.  E2-SS-06  Browse, filter, and pagination API
7.  E2-SS-08  Kafka catalog contract + infrastructure
8.  E2-SS-09  Catalog event publishing
9.  E2-SS-10  Internal gRPC catalog API
10. E2-SS-13  Basic E2E + CI + release

Deferred to Epic 2 Sprint 2 (not part of this merge train):
    E2-SS-07  Bulk import / ingestion pipeline
    E2-SS-11  Full metrics + logging
    E2-SS-12  Full documentation + API artifacts
```

## Merge rules

1. Rebase each ticket branch on the latest default branch before review.
2. A ticket cannot merge unless its direct dependencies are already merged.
3. Require `go test ./...` and `go vet ./...` on every PR.
4. The non-author reviews each P0 ticket.
5. Keep schema and generated-code changes in dedicated commits.
6. Never combine two unfinished tickets to make the build pass.
7. A owns conflict resolution in server composition and Go dependency files.
8. B owns conflict resolution in migrations, Protobuf event schemas, and Docker Compose.

---

# Daily Coordination

## Start of day — 15 minutes

- Confirm the previous day's required merges are complete.
- Identify one blocker per developer at most.
- Agree on API/interface changes before coding.
- Pull the latest default branch.

## Midday integration — 15 minutes

- Push a compiling branch.
- Run focused tests.
- Notify the other developer of contract changes immediately.

## End of day — 30 minutes

- Open or merge the day's PRs.
- Run `go test ./...` on the integrated branch.
- Update ticket acceptance criteria with evidence.
- Move incomplete work explicitly; do not silently mark a partial ticket done.

---

# Risk Controls

| Risk | Response |
|---|---|
| Day 4 is overloaded (three merges plus release, all in one day) | It's designed as a combined integration day, not three independent full tickets — expect close pairing, not parallel independent work; cut E2E scope before cutting correctness (see the Four-Day Timeline note) |
| Admin-key auth model diverges from `music-identity-gatekeeper`'s end-user JWT model | Document explicitly why: catalog writes are operator/ETL actions, not end-user actions, so a shared static key is the simpler, more decoupled choice |
| No consumer exists yet for the Kafka events this service publishes | Same as Epic 1's Kafka topics — preserve the publisher interface and fake tests now; consuming is a later epic's job |
| Artist/album/song schema needs a field nobody anticipated once real data is loaded | Prefer additive migrations over redesigning an already-merged one — especially relevant once Sprint 2's bulk import surfaces real-world data shapes |
| Both developers modify server wiring | B exposes constructors and handlers; A performs all final route and server wiring |
| Deferred tickets (07, 11, 12) get forgotten instead of properly picked up later | Their scope is preserved in this file and in the original seven-day draft; track them explicitly when Epic 2 Sprint 2 is planned, the same way Epic 1's Sprint 2 tracked its own leftovers |
| Scope still exceeds four days even after deferring 07/11/12 | Cut gRPC's `BatchGetSongs` down to a single-ID `GetSong`-only path for this sprint, and restore batching in Sprint 2 — do not cut schema correctness, write-endpoint authorization, or the create→event chain of the E2E test |

---

# Success Metrics

- 10 tickets merged in dependency order (01-06, 08-10, 13)
- Zero known P0 defects
- Every catalog write endpoint rejects a missing or incorrect admin key
- A basic end-to-end flow passes from a clean environment
- HTTP, Kafka, and gRPC contracts work and are tested (full documentation of them is Sprint 2)
- Release `v0.2.0` is reproducible from the tagged commit
- Epic 2 Sprint 2 is scoped and ready to pick up E2-SS-07, 11, and 12
