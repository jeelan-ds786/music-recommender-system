# Epic 1 — Three-Day Closeout Sprint

**Goal:** Close out the User Identity & Listener Profile Service completely — finish everything Sprint 1 left open, add the one piece of EPICS.md's Epic 1 scope Sprint 1 never planned (playlists), and tag the first real release.

**Team:** Developer A and Developer B
**Capacity assumption:** 6-8 focused hours per developer per day
**Sprint length:** 3 consecutive days
**Release target:** `v0.1.0`

---

## Why this sprint exists

Sprint 1 was scoped as a seven-day super sprint and, per its own tracking,
did not fully finish:

- **E1-SS-06** (logout + Redis revocation, Dev A) — implemented on
  `origin/E1-SS-06-Logout-access-token-revocation`, not merged.
- **E1-SS-11** (metrics, structured logging, health) — not started.
- **E1-SS-12** (README/OpenAPI/Postman/architecture diagram) — implemented
  locally on branch `chore/Doc`, not merged.
- **E1-SS-13** (E2E test, CI, release) — not started; `v0.1.0` was never tagged.

Separately, EPICS.md's description of Epic 1 lists *"authentication,
listener profiles, subscription tiers, listening preferences, followed
artists, liked songs, **playlists**, language preferences, and
personalization context"* — and **playlists never appeared anywhere in
Sprint 1's ticket list, migrations, or code**. Every other item in that
list was built; playlists is the one real scope gap, not just an
unfinished ticket.

This sprint finishes both kinds of open item: the four leftover tickets,
and playlists as new scope, then releases `v0.1.0` for the first time.

---

## Recommended ownership

Same split as Sprint 1, continued:

### Developer A
- Finish and merge E1-SS-06 (logout)
- E1-SS-11 (metrics, logging, health)
- Release-captain duties for E1-SS-13

### Developer B
- Finish and merge E1-SS-12 (docs)
- E1-SS-14 (playlist schema, domain contracts, and CRUD) — new
- E1-SS-15 (publish playlist events) — new
- Pair on E1-SS-13, including final documentation corrections for logout and playlists

---

## Definition of Done

Epic 1 is fully complete — matching EPICS.md's description with no
remaining gap — only when:

- Logout revokes both the refresh token and the current access token, and blacklisted access tokens are rejected
- `/metrics`, `/health/live`, and `/health/ready` are live, with structured JSON logs including request ID, method, status, and latency
- A user can create, read, update, and delete playlists, and add/remove songs from them, all behind auth and scoped to the owning user
- `playlist.updated` is published on every playlist mutation, using the same outbox pattern as existing events
- README, OpenAPI, and the Postman collection reflect logout and playlists, not just Sprint 1's original surface
- `go test ./...`, `go vet ./...`, Docker build, migrations, and the end-to-end smoke test pass — the smoke test now covers logout and playlists too
- `v0.1.0` is tagged only after the release branch is green

---

# Tickets

## E1-SS-06 — Logout and access-token revocation (finish and merge)

**Owner:** A
**Priority:** P0
**Estimate:** 3 hours (review, address feedback, merge — implementation already exists)
**Dependencies:** None (already built on `origin/E1-SS-06-Logout-access-token-revocation`)
**Merge position:** 1

### Work

- Rebase `origin/E1-SS-06-Logout-access-token-revocation` on current `main`.
- Address any review feedback; confirm it still compiles and passes tests after the rebase (`main` has moved since this branch was cut — E1-SS-04, 07, 09, 10, and a dependency bump have all landed since).
- Merge.

### Acceptance criteria (unchanged from Sprint 1)

- Logout revokes both the refresh token and current access token.
- The blacklisted access token receives `401` after logout.
- The revoked refresh token cannot rotate.
- Redis keys expire no later than the JWT expiration.
- Raw access or refresh tokens are never written to logs or PostgreSQL.

---

## E1-SS-12 — Documentation and API artifacts (finish and merge)

**Owner:** B
**Priority:** P1
**Estimate:** 3 hours (open PR from `chore/Doc`, merge — implementation already exists)
**Dependencies:** None (already implemented locally on `chore/Doc`, commit `4d23958`)
**Merge position:** 2

### Work

- Open a PR from `chore/Doc` against `main`.
- Rebase if `main` has moved; confirm nothing drifted.
- Merge.
- **Do not** try to add logout or playlist coverage to this PR — that documentation update happens as part of E1-SS-13's final pass, once both have actually merged. Keep this PR scoped to what's already built and verified.

### Acceptance criteria (unchanged from Sprint 1)

- A new developer can start the stack using only the README.
- OpenAPI paths and schemas match handler behavior.
- Postman includes register, login, refresh, OAuth, profile, onboarding, likes, and follows.
- `.env.example` contains placeholders only; `.env` is ignored by Git.

---

## E1-SS-11 — Metrics, structured logging, and health

**Owner:** A
**Priority:** P1
**Estimate:** 8 hours
**Dependencies:** E1-SS-09, E1-SS-10 (both already merged)
**Merge position:** 3

### Work

- Add request-ID middleware and return the ID in a response header.
- Add structured JSON logging with zap.
- Log request ID, user ID when available, method, route, status, and latency.
- Add Prometheus request count and latency metrics.
- Export DB pool statistics and Kafka publish failure/queue metrics.
- Expose `/metrics`, `/health/live`, and `/health/ready`.
- Readiness must check critical dependencies with short timeouts.

### Acceptance criteria (unchanged from Sprint 1)

- No credentials, JWTs, passwords, OAuth codes, or refresh tokens appear in logs.
- Metrics labels do not contain raw URLs, user IDs, or other unbounded values.
- Readiness fails when PostgreSQL is unavailable.
- One HTTP request produces one structured completion log with a request ID.

---

## E1-SS-14 — Playlist schema, domain contracts, and CRUD

**Owner:** B
**Priority:** P0
**Estimate:** 8 hours
**Dependencies:** E1-SS-03 (already merged)
**Merge position:** 4

### Work

- Add migration `000008_create_playlists`: `id UUID PRIMARY KEY DEFAULT gen_random_uuid()`, `user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE`, `name TEXT NOT NULL`, `description TEXT`, `is_public BOOLEAN NOT NULL DEFAULT false`, `created_at`/`updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`.
- Add migration `000009_create_playlist_songs`: `playlist_id UUID REFERENCES playlists(id) ON DELETE CASCADE`, `song_id UUID NOT NULL` (no local foreign key — songs live in the separate catalog service; this is the same cross-service reference pattern already used by `liked_songs.song_id`), `position INT NOT NULL`, `added_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`, `PRIMARY KEY (playlist_id, song_id)`, unique index on `(playlist_id, position)`.
- Implement `internal/playlist`: model, repository (Postgres), service, handler, validator — same layering as `internal/preference`.
- `POST /me/playlists` — `name` required, `description` and `is_public` optional.
- `GET /me/playlists` — list the caller's own playlists, paginated.
- `GET /me/playlists/{id}` — full detail including ordered songs; `404` (not `403`) if the playlist doesn't exist or isn't owned by the caller, so a request can't distinguish "doesn't exist" from "exists but isn't yours."
- `PATCH /me/playlists/{id}` — rename / update description / toggle visibility, using the same pointer-field patch pattern as `PATCH /me`.
- `DELETE /me/playlists/{id}`.
- `POST /me/playlists/{id}/songs/{songID}` — append at the next position; idempotent (adding an already-present song is a no-op, same philosophy as likes/follows).
- `DELETE /me/playlists/{id}/songs/{songID}` — `404` if the song isn't in the playlist.
- Arbitrary reordering (moving a song to a specific position) is out of scope for this sprint — append/remove only. Flag as a follow-up, don't silently build a partial version of it.

### Acceptance criteria

- A playlist cannot be read, patched, or deleted by a user who doesn't own it — structured `404`, not `403` or `500`.
- Adding the same song to a playlist twice is idempotent: no duplicate row, position unchanged.
- Removing a song that isn't in the playlist returns structured `404`.
- Invalid UUID path parameters return `400`.
- Both migrations apply and roll back cleanly on top of `000001`–`000007`.
- Repository queries are parameterized; `go test ./...` passes.

---

## E1-SS-15 — Publish playlist events

**Owner:** B
**Priority:** P0
**Estimate:** 4 hours (reuses the existing outbox/emitter — no new architecture)
**Dependencies:** E1-SS-14, E1-SS-09 (already merged)
**Merge position:** 5

### Work

- Add a `PlaylistUpdated` Protobuf message reusing the standard `EventMetadata` envelope, plus `playlist_id` and `operation` (`CREATED`/`UPDATED`/`DELETED`/`SONG_ADDED`/`SONG_REMOVED`).
- Add a new topic, `user.playlist.updated`, to `kafka-init`'s topic creation.
- Extend `Emitter` with `EmitPlaylistUpdated`; call it from `internal/playlist`'s service after every successful mutation, the same way `internal/preference`'s service already does.
- Reuse the existing outbox-then-direct-publish-with-fallback-relay path unchanged — no new publishing architecture needed.

### Acceptance criteria

- Every successful playlist mutation (create, update, delete, song added, song removed) emits exactly one `user.playlist.updated` event with the correct `operation`.
- Failed database mutations emit no event.
- Unit tests assert event payloads using the existing fake publisher.
- An integration test proves an event reaches the local topic.

---

## E1-SS-13 — End-to-end test, CI, and release

**Owner:** A as release captain; B pairs on failures and final documentation
**Priority:** P0
**Estimate:** 8 hours
**Dependencies:** E1-SS-06, E1-SS-11, E1-SS-12, E1-SS-14, E1-SS-15 (all above)
**Merge position:** 6, always last

### Work

- Extend the end-to-end flow beyond what Sprint 1 covered:
  - migrate database
  - register, login
  - read and patch profile
  - onboarding
  - like and follow
  - create a playlist, add a song, remove a song
  - verify the `user.playlist.updated` events
  - call the gRPC profile API
  - logout
  - verify access-token and refresh-token rejection after logout
- Update the README, OpenAPI spec, and Postman collection (built in E1-SS-12) to add logout and the playlist endpoints — this is the "final documentation corrections" pass, done now that both have actually merged.
- Run tests and race detection in CI.
- Build the Docker image in CI.
- Validate migration up and down against PostgreSQL 16 (now through migration `000009`).
- Run `go vet ./...` and the configured linter.
- Create `v0.1.0` after the default branch is green.

### Acceptance criteria

- CI is green from a clean checkout.
- No test relies on a developer's existing local database state.
- Docker Compose reaches healthy status.
- The full smoke test — including playlists and logout — passes twice consecutively.
- OpenAPI and Postman include logout and every playlist endpoint.
- Tag `v0.1.0` points to the exact green release commit.

---

# Three-Day Timeline

| Day | Developer A | Developer B | Required merges by end of day |
|---|---|---|---|
| **Day 1 — Close the loop** | Rebase, review, and merge E1-SS-06 (logout); start E1-SS-11 | Open PR and merge E1-SS-12 (docs); start E1-SS-14 (playlist schema + create/list/get) | Merge **06**, then **12** |
| **Day 2 — New scope** | Finish and merge E1-SS-11 (metrics/logging) | Finish and merge E1-SS-14 (patch/delete/add-song/remove-song); start E1-SS-15 | Merge **11**, then **14** |
| **Day 3 — Release** | Release captain: E1-SS-13 — extended E2E, CI, doc updates | Finish and merge E1-SS-15 (playlist events); pair on E1-SS-13's doc updates and E2E | Merge **15**, then **13**; tag `v0.1.0` |

---

# Merge Train

```text
1. E1-SS-06  Logout + Redis revocation (finish)
2. E1-SS-12  Documentation + API artifacts (finish)
3. E1-SS-11  Metrics + logging
4. E1-SS-14  Playlist schema + CRUD
5. E1-SS-15  Playlist events
6. E1-SS-13  E2E + CI + release v0.1.0
```

## Merge rules

Same eight rules as Sprint 1 — rebase before review, respect dependency order, `go test`/`go vet` required on every PR, non-author reviews P0 tickets, dedicated commits for schema/generated code, never combine unfinished tickets, A owns server-composition/dependency-file conflicts, B owns migration/Protobuf/Docker-Compose conflicts.

---

# Risk Controls

| Risk | Response |
|---|---|
| E1-SS-06's branch has drifted significantly from current `main` after 5 merged PRs since it was cut | Budget real rebase time on Day 1, not just a fast-forward; if conflicts are large, treat it as a fresh review, not a rubber stamp |
| Playlist scope creeps into reordering, collaborative playlists, or cover-image generation | Explicitly out of scope this sprint — append/remove only, single owner, no collaboration. Note as Epic 1 follow-up work, don't build it now |
| Day 3 becomes overloaded (release + playlist events + doc updates + E2E all landing at once) | E1-SS-15 is intentionally small (4h, pure reuse) specifically so Day 3 has room for the release checklist; if it's still too much, cut the Postman/OpenAPI doc-update scope to logout only and follow up on playlist docs separately — don't cut E2E correctness or delay the tag past a green CI run |
| `v0.1.0` has never been tagged before, so the release process itself is untested | Treat Day 3's tag as the first real dry run of E1-SS-13's own checklist — if something in the checklist doesn't work, fix the checklist, don't skip the step |

---

# Success Metrics

- All 6 tickets in this sprint merged in dependency order
- Epic 1 now matches EPICS.md's description with no remaining scope gap
- Logout, metrics/health, and playlists all covered by tests and the end-to-end smoke test
- README, OpenAPI, and Postman reflect the complete Epic 1 surface, not just Sprint 1's subset
- `v0.1.0` tagged and reproducible from the tagged commit
