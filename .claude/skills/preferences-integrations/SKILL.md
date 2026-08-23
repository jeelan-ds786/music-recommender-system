---
name: preferences-integrations
description: Developer B's Week 2 playbook for the music-identity-gatekeeper service, aligned to Sprint1_TODOS.md — profile CRUD, onboarding, likes/follows, pagination, and Kafka events (tickets E1-SS-03, E1-SS-05, E1-SS-08, E1-SS-09, E1-SS-12). Use when working under internal/profile, internal/preference, internal/event, or migration 005+.
---

# Developer B — Profile, Preferences, and Events

Goal: ship the remaining Week 2 work assigned to Developer B in
`Sprint deliverables/Epic 1/Sprint Tracker/Sprint 1/Sprint1_TODOS.md` —
profile CRUD, onboarding, likes/follows, pagination, and Kafka event
publishing for the `music-identity-gatekeeper` service — without blocking on
or colliding with Developer A's auth/tier/gRPC/metrics work.

Module: `github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper`
Router: `go-chi/chi/v5` (see `cmd/server/main.go`)

This skill previously described a different, conflicting plan (a "Person 2"
role that also owned gRPC, a plural `internal/preferences/` package, and a
single `feature/preferences-integrations` branch for the whole week). That
plan has been superseded by `Sprint1_TODOS.md`, which this project actually
follows. The corrections below fold in what was actually built for E1-SS-01.

## Ticket scope (owner: B, per Sprint1_TODOS.md)

| Ticket | What | Depends on |
|---|---|---|
| E1-SS-01 | Preferences migration + domain contracts | — (**done** — see below) |
| E1-SS-03 | Full profile GET/PATCH | E1-SS-01 |
| E1-SS-05 | Onboarding, likes/follows, pagination | E1-SS-03 |
| E1-SS-08 | Kafka contract + local infrastructure | E1-SS-01 |
| E1-SS-09 | Publish registration/preference events | E1-SS-05, E1-SS-08 |
| E1-SS-12 | README/OpenAPI/Postman/architecture diagram | E1-SS-05, E1-SS-07, E1-SS-09, E1-SS-10 |

**Not B's work:** tier claims/authorization (E1-SS-04), logout/Redis
revocation (E1-SS-06), Google OAuth (E1-SS-07), the internal **gRPC** API
(E1-SS-10), and metrics/logging (E1-SS-11) all belong to Developer A. Don't
build gRPC as part of this skill — coordinate with A on the
`GetListenerProfile` response shape instead, since it will read data this
skill owns.

## What E1-SS-01 already built (don't redo this)

- Migration `000004_create_preferences` — a `preferences` table keyed on
  `user_id` (FK to `users(id) ON DELETE CASCADE`), with four array columns
  (`liked_song_ids`, `followed_artist_ids`, `genre_seeds`,
  `language_prefs`, all `NOT NULL DEFAULT '{}'`) and GIN indexes on the two
  ID arrays. Verified: applies and rolls back cleanly, and a bare
  `INSERT INTO preferences (user_id)` correctly defaults all four arrays to
  empty.
- `internal/preference/` (**singular**, not `internal/preferences/`):
  `model.go` (`Preference`), `repository.go` (`Repository` interface +
  `PostgresRepository` with `Create`/`GetByUserID`), `service.go`
  (`Service` interface, currently just `GetPreferences`), `dto.go`
  (`PreferenceResponse`), `error.go` (`ErrPreferenceNotFound`). No HTTP,
  Kafka, or gRPC imports anywhere in this package — keep it that way.
- Tests: `internal/preference/service_test.go` (unit, fake repository, no
  DB needed) and `internal/preference/repository_test.go` (integration,
  needs `DB_URL`, self-skips if unset). Run with `make test`.

**Migration 005 is the next free number.** E1-SS-05's own engineering note
in Sprint1_TODOS.md says UUID arrays don't support reliable cursor
pagination under concurrent writes — add normalized
`liked_songs(user_id, song_id, created_at)` and
`followed_artists(user_id, artist_id, created_at)` tables (unique
constraints on `(user_id, song_id)` / `(user_id, artist_id)`) in migration
`000005_*` when you get to E1-SS-05, rather than trying to paginate the
existing arrays directly. Keep the summary arrays on `preferences` only if
a downstream ML query genuinely needs them.

## File ownership

```
internal/profile/       you (E1-SS-03)
internal/preference/    you (extend the existing package — see above)
internal/event/         you (E1-SS-08/09; singular, matches Sprint1_TODOS.md wording)
migrations/000005_*      you (and any migration after 005)
```

Developer A owns `internal/auth/`, `internal/token/`, middleware, and
server composition (`cmd/server/main.go`). If a task seems to need a change
inside those paths, stop and coordinate instead of editing them directly.

**Shared-file rule (per Sprint1_TODOS.md):**
- `cmd/server/main.go`, `go.mod`, `go.sum` — A is the merge owner. You may
  update dependencies on a feature branch, but don't wire routes into
  `main.go` yourself: expose constructors and handlers, and let A wire them
  in during integration.
- `docker-compose.yml` — you own this during the sprint (Kafka goes here
  for E1-SS-08).

## Existing conventions to reuse (don't reinvent)

- Auth middleware sets the caller's user id in context:
  `auth.UserIDFromContext(ctx)` (`internal/auth/middleware.go`) returns
  `(string, bool)`. Pull `user_id` from there in your handlers.
- Tier claims land on the JWT via E1-SS-04 (A's ticket) — agree with A on
  the exact context key/claim shape before gating any of your routes on
  tier.
- Error responses: `internal/response.Error(w, status, code)` and
  `internal/response.ValidationError(w, status, code, field)` for errors;
  `internal/response.JSON(w, status, data)` for success payloads. Reuse
  these rather than hand-rolling JSON bodies.
- Route registration: `main.go` currently wires `/auth` and `/me` inline.
  Expose a `RegisterRoutes(router chi.Router, deps ...)` function per
  package (`profile.RegisterRoutes`, etc.) so A can mount yours without
  editing your code.
- **Logging** (built this session, already wired into `cmd/server/main.go`):
  - `internal/logger` — a leveled logger (`logger.New(level)`, methods
    `.Debug`/`.Info`/`.Error`). Level comes from the `LOG_LEVEL` env var
    (`debug`/`info`/`error`/`none`, defaults to `info`).
  - `internal/db.NewPostgresPool(ctx, dsn, tracer)` takes a
    `pgx.QueryTracer`; `internal/db.NewQueryTracer(log)` logs every SQL
    query as `READ`/`WRITE` with bound arg values and duration at Info
    level. Any repository you add gets this logging for free — no changes
    needed in your repository code.
  - `internal/reqid` — chi middleware (`reqid.Middleware`, already applied
    via `r.Use(...)` in `main.go`) assigns/propagates an `X-Request-ID`
    header; read it in a service with `reqid.FromContext(ctx)` to tag your
    own log lines the way `auth.Register` does.
- **Local dev commands:**
  - `docker compose up -d postgres redis` (repo root) — starts local
    Postgres 16 (`muse_identity` db) and Redis 7, matching the credentials
    in the root `docker-compose.yml`.
  - `make migrate-up` / `make migrate-down` (inside
    `music-identity-gatekeeper/`) — needs the `golang-migrate` CLI and a
    `.env` with `DB_URL` set.
  - `make dev` — runs `go run ./cmd/server` (equivalent of `npm run dev`).
  - `make test` — runs `go test ./internal/preference/... -v`; extend this
    target's path (or add a new one) as you add tests under
    `internal/profile/` and `internal/event/`.
  - **Makefile gotcha:** the `Makefile` does `include .env` + `export`,
    which makes `.env`'s values win over a shell-prefixed override (e.g.
    `LOG_LEVEL=debug make dev` will NOT override `.env`). To override at
    the command line, either put the var after `make`
    (`make dev LOG_LEVEL=debug`) or add `-e`
    (`LOG_LEVEL=debug make -e dev`).
- No tests existed anywhere in the repo before E1-SS-01; `internal/preference`
  now has the first ones. Follow the same pattern (table-driven where it
  fits, a fake for pure unit tests, a DB-backed test that self-skips
  without `DB_URL`) for `internal/profile` and `internal/event`.

## Branching and merge order

Sprint1_TODOS.md's Merge Train is explicit: **short-lived branches, one per
ticket, merged continuously** — not one big branch for the whole week.
Name branches after the ticket, e.g. `feature/e1-ss-03-profile-get-patch`,
`feature/e1-ss-05-onboarding-likes-follows`, `feature/e1-ss-08-kafka-contract`.

Merge order follows the numbered Merge Train in Sprint1_TODOS.md. For your
tickets specifically: **03 → 05 → 08 → 09 → 12**, and each can only merge
once its listed dependencies are already merged. Don't merge 05 before 03,
or 09 before both 05 and 08 are in.

## Day-by-day (per Sprint1_TODOS.md's Seven-Day Timeline)

- **Day 1** — E1-SS-01 preferences schema and domain contracts. **Done.**
- **Day 2** — E1-SS-03: profile repositories (users, listener_profiles,
  preferences), full `GET /me`, `PATCH /me` with pointer/patch fields so
  omitted values aren't overwritten, uppercase ISO 3166-1 alpha-2 country
  validation, profile/preference row creation on registration.
- **Day 3** — E1-SS-05: `POST /me/onboarding` (idempotent or `409` on
  resubmission), like/unlike songs, follow/unfollow artists (atomic,
  idempotent), migration `000005_*` normalized tables, cursor pagination
  for `GET /me/likes/songs`.
- **Day 4** — E1-SS-08: Protobuf for `user.preference.updated` and
  `user.registered` (with `event_id`, `event_type`, `schema_version`,
  `occurred_at`, `user_id`), `segmentio/kafka-go` behind a `Publisher`
  interface, Kafka added to `docker-compose.yml`, topic init, a no-op
  publisher for unit tests.
- **Day 5** — E1-SS-09: publish `user.registered` after registration and
  `user.preference.updated` after onboarding/like/unlike/follow/unfollow;
  bounded async publisher with shutdown draining; document delivery
  semantics (outbox follow-up if DB/event atomicity is required).
- **Day 6** — E1-SS-12: README, `.env.example` (placeholders only), OpenAPI
  3.0 for every endpoint, updated Postman collection, Mermaid architecture
  diagram (HTTP, gRPC, Postgres, Redis, Kafka).
- **Day 7** — Pair with A (release captain) on E1-SS-13: end-to-end flow,
  Docker/Kafka verification, documentation corrections. Merge **13** last,
  and only after CI is green.

## Definition of done (your tickets)

- Profile GET/PATCH, onboarding, likes/follows, and pagination all working
  and idempotent per each ticket's acceptance criteria in
  `Sprint1_TODOS.md`.
- Kafka producer publishing `user.registered` and `user.preference.updated`
  on every documented mutation; failed DB mutations emit no event.
- Repository, handler, and (mocked + integration) Kafka tests passing.
- Migration `000005` (and any further migrations you add) apply and roll
  back cleanly on top of `000001`–`000004`.
- No edits to `internal/auth/`, `internal/token/`, or `cmd/server/main.go`
  route wiring — expose constructors/handlers for A to wire in.
- README/OpenAPI/Postman/architecture diagram match the implementation
  (E1-SS-12).
