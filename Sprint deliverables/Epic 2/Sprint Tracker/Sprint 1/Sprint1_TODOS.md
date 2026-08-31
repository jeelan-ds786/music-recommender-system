# Epic 2 Sprint 1 — Six-Day Combined Sprint

**Goal:** Close out Epic 1's remaining work (Phase 1, Days 1-2), then build and release the core Music Catalog Management Service (Phase 2, Days 3-6).

**Team:** Developer A and Developer B
**Capacity assumption:** 6-8 focused hours per developer per day
**Sprint length:** 6 consecutive days
**Release targets:** `v0.1.0` (end of Day 2), then `v0.2.0` (end of Day 6)

> There is no separate "Epic 1 Sprint 2" anymore — its remaining tickets
> now run as Phase 1 of this sprint, per your direction. This is the one
> authoritative plan.

---

## E1-SS-06 and E1-SS-12: resolved, not scheduled work

Neither ticket consumes any developer time in this sprint — both are
merge-only at this point:

**E1-SS-06 — actually resolved.** All 6 real conflicts (`Makefile`,
`cmd/server/main.go`, `internal/auth/middleware.go`,
`internal/preference/service.go` + test, `internal/token/service.go`)
are fixed, committed as a merge commit on local branch
`fix/e1-ss-06-resolve-conflicts` (`5a83591`, based on `main`). Verified:
`go build ./...`, `go vet ./...`, and `go test -race ./...` all pass,
including the DB-backed tests against a real local Postgres (previously
these could only be checked for conflicts, not correctness). Also ran a
live smoke test: register → login → `GET /me` (200) → logout (204) →
`GET /me` with the same access token (401 `REVOKED_ACCESS_TOKEN`) →
refresh with the same refresh token (401). Redis blacklist TTL measured
at 882s, correctly under the access token's 900s lifetime. No raw token
appeared in logs.
**Not yet merged into `main` or pushed to `origin`** — say the word and
I'll do either or both; I stopped short of pushing/merging on my own
since that touches shared state.

**E1-SS-12 — done, pending merge.** `chore/Doc` is verified clean (zero
conflicts against `main`). It just needs pushing to `origin` and merging
via PR — no further work or discussion needed.

---

## Remaining Epic 1 tickets (moved into this sprint as Phase 1)

Two tickets per developer, apart from the two above:

- **Developer A:** E1-SS-11 (metrics/logging/health), E1-SS-13 (Epic 1 end-to-end test, CI, and release `v0.1.0` — release captain)
- **Developer B:** E1-SS-14 (playlist schema + CRUD), E1-SS-15 (playlist events)

This is real remaining work — the 3-day estimate from the old separate
plan shrinks to about 2 days now that 06 and 12 no longer occupy Day 1.

---

## Starting point for Phase 2 (Epic 2)

Greenfield service, reusing Epic 1's proven patterns rather than
inventing new ones — same stack (Go / `chi` / `pgx/v5` / `golang-migrate`
/ `segmentio/kafka-go` / `google.golang.org/grpc`), same repo-root
`docker-compose.yml` (adds a `catalog-svc` block + `muse_catalog`
database), same handler → service → repository layering and error-shape
conventions, same transactional-outbox Kafka publishing design proven in
E1-SS-09 (reimplemented, not shared as a library).

**Deferred to a future Epic 2 Sprint 2** (unchanged from the prior draft,
scope preserved, not dropped): E2-SS-07 (bulk import), E2-SS-11 (full
observability), E2-SS-12 (full docs/OpenAPI/Postman/diagram).

---

## Recommended ownership

### Developer A
- Phase 1: E1-SS-11, release-captain E1-SS-13
- Phase 2: E2-SS-02, E2-SS-04, E2-SS-06, E2-SS-10, release-captain E2-SS-13
- Shared integration files: `cmd/server/main.go`, `go.mod`, `go.sum`, `catalog-svc`'s block in root `docker-compose.yml`

### Developer B
- Phase 1: E1-SS-14, E1-SS-15, pair on E1-SS-13
- Phase 2: E2-SS-01, E2-SS-03, E2-SS-05, E2-SS-08, E2-SS-09
- Migrations, event Protobuf definitions, Kafka/Docker Compose additions

---

## Definition of Done

**Phase 1 (Epic 1 fully complete, matching EPICS.md):**
- Logout revokes both tokens; blacklisted access tokens rejected (already verified above)
- `/metrics`, `/health/live`, `/health/ready` live with structured JSON logs
- Playlists: create/read/update/delete + add/remove songs, ownership-scoped, idempotent
- `playlist.updated` published on every playlist mutation
- README/OpenAPI/Postman reflect logout and playlists
- `v0.1.0` tagged only after a green release branch

**Phase 2 (Epic 2 core):**
- Artist/album/song CRUD, browse/filter, real FKs enforced
- Every mutation admin-key protected; every read public
- `catalog.entity.updated` published on every mutation
- gRPC `GetArtist`/`GetSong`/`BatchGetSongs` work internally
- Basic health check + minimal README (full observability/docs are Epic 2 Sprint 2)
- `v0.2.0` tagged only after a green release branch

---

# Phase 1 — Epic 1 Remaining Work (Days 1-2)

## E1-SS-11 — Metrics, structured logging, and health

**Owner:** A | **Priority:** P1 | **Estimate:** 8 hours | **Dependencies:** E1-SS-09, E1-SS-10 (merged) | **Merge position:** 1

Request-ID middleware, zap JSON logging (request ID, user ID, method, route, status, latency), Prometheus request/latency/DB-pool/Kafka-error metrics, `/metrics`, `/health/live`, `/health/ready`.

**Acceptance:** no secrets/tokens in logs; metrics labels bounded; readiness fails when Postgres is down; one request produces one structured completion log.

---

## E1-SS-14 — Playlist schema, domain contracts, and CRUD

**Owner:** B | **Priority:** P0 | **Estimate:** 8 hours | **Dependencies:** E1-SS-03 (merged) | **Merge position:** 2

- Migration `000008_create_playlists` (`id`, `user_id` FK cascade, `name`, `description`, `is_public`, timestamps).
- Migration `000009_create_playlist_songs` (`playlist_id` FK cascade, `song_id` — no local FK, cross-service reference like `liked_songs.song_id` — `position`, `added_at`; PK `(playlist_id, song_id)`; unique index on `(playlist_id, position)`).
- `internal/playlist`: model/repository/service/handler/validator, same layering as `internal/preference`.
- `POST/GET /me/playlists`, `GET/PATCH/DELETE /me/playlists/{id}` (pointer-field patch), `POST/DELETE /me/playlists/{id}/songs/{songID}` (idempotent append, 404 on missing remove).
- Ownership check returns `404` (not `403`) for a non-owned playlist. Arbitrary reordering out of scope.

**Acceptance:** non-owner gets `404`; duplicate add is idempotent; missing remove is `404`; invalid UUIDs are `400`; both migrations apply/roll back cleanly on top of `000001`-`000007`.

---

## E1-SS-15 — Publish playlist events

**Owner:** B | **Priority:** P0 | **Estimate:** 4 hours (pure reuse) | **Dependencies:** E1-SS-14, E1-SS-09 (merged) | **Merge position:** 3

`PlaylistUpdated` Protobuf message (standard envelope + `playlist_id` + `operation`), new topic `user.playlist.updated`, `Emitter.EmitPlaylistUpdated` called from `internal/playlist`'s service after every mutation. No new publishing architecture.

**Acceptance:** every mutation emits exactly one event with the correct `operation`; failed mutations emit none; unit tests use the existing fake publisher; integration test proves delivery.

---

## E1-SS-13 — End-to-end test, CI, and release `v0.1.0`

**Owner:** A as release captain; B pairs | **Priority:** P0 | **Estimate:** 8 hours | **Dependencies:** E1-SS-06 (merge only), E1-SS-11, E1-SS-12 (merge only), E1-SS-14, E1-SS-15 | **Merge position:** 4, closes Phase 1

Extended E2E: migrate → register/login → profile GET/PATCH → onboarding → like/follow → playlist create/add-song/remove-song → verify playlist events → gRPC profile call → logout → verify token rejection. Update README/OpenAPI/Postman for logout and playlists. CI green, migration up/down through `000009`, tag `v0.1.0`.

**Acceptance:** CI green from clean checkout; smoke test (incl. playlists, logout) passes twice; OpenAPI/Postman include logout and every playlist endpoint; tag points to the exact green commit.

---

# Phase 2 — Epic 2 Catalog Core (Days 3-6)

## E2-SS-01 — Catalog schema and domain contracts

**Owner:** B | **Priority:** P0 | **Estimate:** 6 hours | **Dependencies:** None | **Merge position:** 5

Migrations `000001_create_artists`, `000002_create_albums`, `000003_create_songs` (`genre_tags`/`mood_tags` arrays with GIN indexes, `acoustic_features JSONB` reserved for Epic 6, `external_id` reserved for Sprint 2's bulk import), `000004_create_song_featured_artists` (normalized join table, not an array). `internal/artist`, `internal/album`, `internal/song` domain models/repositories/services, no transport imports.

**Acceptance:** migrations apply/roll back cleanly; nonexistent-artist song creation fails at the DB level; `go test ./...` passes.

---

## E2-SS-02 — Service scaffolding and admin authorization

**Owner:** A | **Priority:** P0 | **Estimate:** 7 hours | **Dependencies:** None | **Merge position:** 6

`cmd/server/main.go`, `internal/response`, `internal/db` (copy Epic 1's shapes). Dockerfile, `catalog-svc` + `muse_catalog` in root `docker-compose.yml`. `ADMIN_API_KEY`/`X-Admin-Key` write authorization (not JWTs — catalog writes are operator/ETL actions). All `GET`s public. Basic `/health/live`, `/health/ready`.

**Acceptance:** boots with placeholder env vars; `AdminKeyMiddleware` returns `401` for missing/incorrect key; `go vet`/`go test -race` pass.

---

## E2-SS-03 — Artist CRUD and full read API

**Owner:** B | **Priority:** P0 | **Estimate:** 6 hours | **Dependencies:** E2-SS-01 | **Merge position:** 7

Full CRUD, public reads, admin-key writes, pointer-field patch, cascading delete documented as destructive.

**Acceptance:** 401/400/404 patterns match Epic 1's conventions; parameterized queries.

---

## E2-SS-04 — Album CRUD and artist-album linking

**Owner:** A | **Priority:** P0 | **Estimate:** 6 hours | **Dependencies:** E2-SS-01, E2-SS-03 | **Merge position:** 8

`POST /albums` validates `artist_id` exists (structured `404`, not a raw FK error); `GET /artists/{id}/albums` paginated.

**Acceptance:** same shape as E2-SS-03.

---

## E2-SS-05 — Song CRUD and catalog metadata

**Owner:** B | **Priority:** P0 | **Estimate:** 7 hours | **Dependencies:** E2-SS-01, E2-SS-03, E2-SS-04 | **Merge position:** 9

Full CRUD incl. `song_featured_artists` join-table writes; `genre_tags`/`mood_tags` capped at 5; `popularity_score` read-only (a later epic owns writing it).

**Acceptance:** nonexistent/mismatched artist-album returns structured error; join-table PK prevents duplicate features.

---

## E2-SS-06 — Catalog browse, filter, and pagination API

**Owner:** A | **Priority:** P0 | **Estimate:** 6 hours | **Dependencies:** E2-SS-05 | **Merge position:** 10

`GET /catalog/songs` with genre/mood/artist/year filters, allowlisted sort, cursor pagination (same opaque-cursor approach as Epic 1). Full-text/semantic search is explicitly Epic 12's job.

---

## E2-SS-08 — Kafka catalog event contract and local infrastructure

**Owner:** B | **Priority:** P0 | **Estimate:** 6 hours | **Dependencies:** E2-SS-01 | **Merge position:** 11

`CatalogEntityUpdated` Protobuf (standard envelope + `entity_type`/`entity_id`/`operation`); `Publisher` interface; extend `kafka-init` with the new topic.

**Acceptance:** reproducible codegen; unit tests don't need Kafka.

---

## E2-SS-09 — Publish catalog mutation events

**Owner:** B | **Priority:** P0 | **Estimate:** 7 hours | **Dependencies:** E2-SS-05, E2-SS-08 | **Merge position:** 12

Reuse the outbox-then-direct-publish-with-fallback-relay design proven (and hard-learned) in E1-SS-09.

**Acceptance:** every mutation emits the right event; disabling the relay doesn't disable direct publish (verify explicitly).

---

## E2-SS-10 — Internal gRPC catalog API

**Owner:** A | **Priority:** P0 | **Estimate:** 7 hours | **Dependencies:** E2-SS-05, E2-SS-06 | **Merge position:** 13

`GetArtist`/`GetSong`/`BatchGetSongs` on internal port `50052` (`50051` is identity's). `BatchGetSongs` partially succeeds on mixed valid/invalid/missing IDs. Graceful shutdown alongside HTTP.

---

## E2-SS-13 — Basic end-to-end test, CI, and release `v0.2.0`

**Owner:** A as release captain; B pairs | **Priority:** P0 | **Estimate:** 6 hours | **Dependencies:** E2-SS-01 through 06, 08, 09, 10 | **Merge position:** 14, closes Phase 2

Basic E2E: migrate → create artist/album/song → browse finds it → event verified → gRPC `BatchGetSongs` returns it → update/delete → events verified → `404` after delete. Minimal README stub. Tag `v0.2.0` after green CI.

---

# Six-Day Timeline

| Day | Phase | Developer A | Developer B | Merges | Release |
|---|---|---|---|---|---|
| **1** | 1 | E1-SS-11 metrics/logging | E1-SS-14 playlist schema + CRUD | 14, then 11 | — |
| **2** | 1 | Release captain: E1-SS-13 (includes merging E1-SS-06 and E1-SS-12 first) | Finish E1-SS-15; pair on E1-SS-13 | 06 (merge-only), 12 (merge-only), 15, then 13 | **`v0.1.0`** |
| **3** | 2 | E2-SS-02 scaffolding + admin auth | E2-SS-01 catalog schema | 01, then 02 | — |
| **4** | 2 | E2-SS-04 album CRUD | E2-SS-03 artist CRUD | 03, then 04 | — |
| **5** | 2 | E2-SS-06 browse/filter | E2-SS-05 song CRUD; start E2-SS-08 | 05, then 06 | — |
| **6** | 2 | E2-SS-10 gRPC; pair as release captain | Finish 08; E2-SS-09; pair | 08, 09, 10, 13 | **`v0.2.0`** |

**On the freed-up day:** this plan is one day shorter than the last draft
(6 vs. 7) purely because E1-SS-06 and E1-SS-12 turned out not to need
scheduled dev time. If you'd rather keep a 7-day cadence — e.g. to give
Day 6 (still the busiest day, three merges plus release) some slack — say
so and I'll add it back as a buffer day between Phase 1 and Phase 2
rather than manufacturing work to fill it.

---

# Merge Train

```text
—   E1-SS-06  Logout (resolved, merge-only — on fix/e1-ss-06-resolve-conflicts)
—   E1-SS-12  Docs (verified clean, merge-only — on chore/Doc)
1.  E1-SS-14  Playlist schema + CRUD
2.  E1-SS-11  Metrics + logging
3.  E1-SS-15  Playlist events
4.  E1-SS-13  Epic 1 E2E + CI + release v0.1.0
5.  E2-SS-01  Catalog schema + domain contracts
6.  E2-SS-02  Service scaffolding + admin authorization
7.  E2-SS-03  Artist CRUD + full read API
8.  E2-SS-04  Album CRUD + artist linking
9.  E2-SS-05  Song CRUD + catalog metadata
10. E2-SS-06  Browse, filter, and pagination API
11. E2-SS-08  Kafka catalog contract + infrastructure
12. E2-SS-09  Catalog event publishing
13. E2-SS-10  Internal gRPC catalog API
14. E2-SS-13  Epic 2 basic E2E + CI + release v0.2.0

Deferred (not part of this merge train): E2-SS-07, E2-SS-11, E2-SS-12
```

## Merge rules

1. Rebase each ticket branch on the latest default branch before review.
2. A ticket cannot merge unless its direct dependencies are already merged — E1-SS-13 specifically needs E1-SS-06 and E1-SS-12 actually landed on `main` first, not just resolved/verified on their own branches.
3. Require `go test ./...` and `go vet ./...` on every PR.
4. The non-author reviews each P0 ticket.
5. Keep schema and generated-code changes in dedicated commits.
6. Never combine two unfinished tickets to make the build pass.
7. A owns conflict resolution in server composition and Go dependency files.
8. B owns conflict resolution in migrations, Protobuf event schemas, and Docker Compose.

---

# Risk Controls

| Risk | Response |
|---|---|
| E1-SS-06's resolved branch drifts again before it's actually merged into `main` | Merge it soon — every day it sits unmerged risks another PR landing on `main` and reopening the same conflict surface |
| Playlist scope creeps into reordering or collaboration | Explicitly out of scope — append/remove only |
| Day 6 repeats the earlier crunch (three merges + release in one day) | Paired integration day, not three solo tickets; cut E2E scope (update/delete legs) before cutting correctness; consider the freed-up day as a buffer here if it's still too tight |
| Deferred Epic 2 tickets (07, 11, 12) get forgotten | Scope preserved above under Phase 2's starting-point note |

---

# Success Metrics

- 14 tickets merged in dependency order, plus E1-SS-06 and E1-SS-12 actually landed on `main`
- `v0.1.0` tagged at Phase 1's close, `v0.2.0` at Phase 2's close
- Epic 1 fully matches EPICS.md's description; Epic 2's core is live
- Epic 2 Sprint 2 is scoped and ready to pick up E2-SS-07, 11, and 12
