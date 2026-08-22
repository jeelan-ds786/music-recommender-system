# Epic 1 — Seven-Day Super Sprint

**Goal:** Finish the User Identity & Listener Profile Service in seven days with two developers.

**Team:** Developer A and Developer B  
**Capacity assumption:** 6-8 focused hours per developer per day  
**Sprint length:** 7 consecutive days  
**Release target:** `v0.1.0`

---

## Starting point

The repository already contains:

- User, refresh-token, and listener-profile migrations
- Register, login, and refresh endpoints
- bcrypt password hashing
- HS256 access tokens with a 15-minute TTL
- Hashed refresh tokens and token rotation
- Bearer-token authentication middleware
- A protected `GET /me` route that currently returns only `user_id`
- Docker, PostgreSQL, Redis, Makefile, and Postman foundations

This sprint finishes the missing Week 1 work and all Week 2 work. Existing code should be hardened and tested, not rewritten without a failing test or a clear defect.

---

## Recommended ownership

### Developer A — Identity, security, and runtime owner

Developer A owns:

- Auth hardening and auth tests
- JWT tier claims and authorization middleware
- Logout and Google OAuth2
- HTTP/gRPC server composition
- Prometheus, logging, request IDs, and release integration
- Shared integration files such as `cmd/server/main.go` and final dependency reconciliation

### Developer B — Profile, preferences, and event owner

Developer B owns:

- Preferences migration and profile repositories
- Profile CRUD and onboarding
- Like/unlike and follow/unfollow APIs
- Kafka event schema and publisher
- Profile and preference tests
- Kafka additions to Docker Compose

### Why this split minimizes dependencies

A works mainly in `internal/auth`, `internal/token`, middleware, and server composition. B works mainly in new `internal/profile`, `internal/preference`, `internal/event`, migration, and Protobuf files. The two developers should not both edit `cmd/server/main.go`; B exposes constructors and route handlers, then A wires them into the server during integration.

**Shared-file rule:** A is the merge owner for `cmd/server/main.go`, `go.mod`, and `go.sum`. B may update dependencies in a feature branch, but A resolves the final shared-file changes. B owns `docker-compose.yml` during this sprint.

---

## Definition of Done

Epic 1 is complete only when:

- All HTTP endpoints in the Epic 1 API surface are implemented
- Refresh rotation and logout revocation are covered by tests
- Google OAuth callback handles account linking safely
- Profile, onboarding, tier, likes, follows, and pagination work behind auth
- Preference mutations publish the expected Kafka event
- `user.registered` is published after successful registration
- gRPC `GetListenerProfile` works on port `50051`
- `/metrics` exposes request, latency, DB-pool, and Kafka-error metrics
- Logs are structured JSON and include request ID, user ID, method, status, and latency
- `go test ./...`, `go vet ./...`, Docker build, migrations, and the end-to-end smoke test pass
- README, OpenAPI, architecture diagram, and Postman collection match the implementation
- The release is tagged `v0.1.0` only after the release branch is green

---

# Tickets

## E1-SS-01 — Preferences schema and domain contracts

**Owner:** B  
**Priority:** P0  
**Estimate:** 6 hours  
**Dependencies:** None  
**Merge position:** 1

### Work

- Add migration `000004_create_preferences` with:
  - `user_id UUID PRIMARY KEY` referencing `users(id)` with `ON DELETE CASCADE`
  - `liked_song_ids UUID[] NOT NULL DEFAULT '{}'`
  - `followed_artist_ids UUID[] NOT NULL DEFAULT '{}'`
  - `genre_seeds TEXT[] NOT NULL DEFAULT '{}'`
  - `language_prefs TEXT[] NOT NULL DEFAULT '{}'`
  - `updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`
- Add GIN indexes for `liked_song_ids` and `followed_artist_ids`.
- Add reversible down migration.
- Define profile and preference models, repository interfaces, and service interfaces.
- Define request/response DTOs used by HTTP and gRPC without implementing transport logic yet.

### Acceptance criteria

- Migration applies and rolls back cleanly.
- A preference row can be created for a user and defaults to empty arrays.
- Domain contracts compile and do not import HTTP, Kafka, or gRPC packages.
- `go test ./...` passes.

---

## E1-SS-02 — Auth hardening and table-driven tests

**Owner:** A  
**Priority:** P0  
**Estimate:** 8 hours  
**Dependencies:** None  
**Merge position:** 2; may be reviewed in parallel with E1-SS-01

### Work

- Add table-driven tests for register and login:
  - successful registration
  - duplicate email
  - invalid email
  - weak password
  - successful login
  - wrong password
  - unknown email
- Use a valid dummy bcrypt hash when an email is unknown so login still performs a password comparison.
- Normalize email consistently before lookup and insert.
- Ensure unknown email and wrong password return the same status and error code.
- Standardize validation responses with machine-readable field errors.
- Add JWT tests for valid, expired, malformed, wrong-algorithm, and tampered tokens.
- Add refresh tests for successful rotation and rejected reuse of a revoked token.

### Acceptance criteria

- Auth tests do not require a running HTTP server.
- Unknown-user login follows the bcrypt comparison path.
- No response reveals whether an email exists.
- Old refresh tokens cannot be reused after rotation.
- `go test -race ./internal/auth/... ./internal/token/...` passes.

---

## E1-SS-03 — Full profile read and partial update

**Owner:** B  
**Priority:** P0  
**Estimate:** 8 hours  
**Dependencies:** E1-SS-01  
**Merge position:** 3

### Work

- Implement repositories that read users, listener profiles, and preferences.
- Replace the minimal `GET /me` response with the complete profile response.
- Implement `PATCH /me` for `display_name`, `avatar_url`, `country`, `language`, and `birth_year`.
- Use pointer fields or an equivalent patch model so omitted values are not overwritten.
- Validate country as an uppercase ISO 3166-1 alpha-2 value.
- Create profile and preference rows during registration or lazily in one well-documented transaction.
- Add handler and repository tests.

### Acceptance criteria

- Unauthenticated requests return structured `401` responses.
- `GET /me` returns account, profile, tier, and preference fields.
- Patching one field does not change any omitted field.
- Invalid country and birth year return structured `400` responses.
- Repository queries are parameterized.

---

## E1-SS-04 — Tier claims and premium authorization

**Owner:** A  
**Priority:** P0  
**Estimate:** 6 hours  
**Dependencies:** E1-SS-01 and the profile repository contract from E1-SS-03  
**Merge position:** 4

### Work

- Add `GetTier(userID)` and `Upgrade(userID, tier)` service behavior.
- Put `tier`, not `auth_provider`, in the access-token authorization claims. The auth provider may remain as a separate claim if needed.
- Add typed context helpers for both user ID and tier.
- Add `TierMiddleware` that returns structured `403` for insufficient tier.
- Reissue tokens after an upgrade; document that existing access tokens retain their old claim until refreshed or reissued.
- Add tests for free-user denial, premium-user access, invalid tier, and upgraded token claims.

### Acceptance criteria

- JWT parsing validates HS256 and expiration.
- A free token cannot call a premium-only test route.
- A token issued after upgrade contains the new tier.
- No middleware trusts a tier supplied through headers or request bodies.

---

## E1-SS-05 — Onboarding and preference mutations

**Owner:** B  
**Priority:** P0  
**Estimate:** 10 hours  
**Dependencies:** E1-SS-03  
**Merge position:** 5

### Work

- Implement `POST /me/onboarding` with up to five genre seeds, language preferences, and optional followed artist IDs.
- Make onboarding write behavior explicit: reject a second submission with `409`, or make it idempotent. Use `409` unless product requirements say otherwise.
- Implement:
  - `POST /me/likes/songs/:id`
  - `DELETE /me/likes/songs/:id`
  - `GET /me/likes/songs`
  - `POST /me/following/artists/:id`
  - `DELETE /me/following/artists/:id`
- Make like and follow operations atomic and idempotent.
- Implement stable cursor pagination for liked song IDs. Do not paginate directly over an unordered PostgreSQL array; define a deterministic order or use a normalized interaction table.

### Acceptance criteria

- A song or artist cannot appear twice.
- Unlike/unfollow of a missing item returns the documented `404` response.
- Invalid UUID path parameters return `400`.
- Pagination has no duplicates or omissions in the tested stable dataset.
- Concurrent duplicate-like requests leave one stored value.
- Tests cover onboarding limits and all mutation edge cases.

### Engineering note

The Week 2 plan proposes UUID arrays and cursor pagination together. Arrays do not preserve a durable insertion identity for reliable cursor pagination under concurrent writes. For a production-quality result, add normalized `liked_songs(user_id, song_id, created_at)` and `followed_artists(user_id, artist_id, created_at)` tables with unique constraints. Keep summary arrays only if a downstream ML query truly requires them.

---

## E1-SS-06 — Logout and access-token revocation

**Owner:** A  
**Priority:** P0  
**Estimate:** 6 hours  
**Dependencies:** E1-SS-02  
**Merge position:** 6; can be developed in parallel with E1-SS-05

### Work

- Add a unique JWT ID (`jti`) to access tokens.
- Implement `POST /auth/logout` behind bearer authentication.
- Hash and revoke the submitted refresh token in PostgreSQL.
- Store `blacklist:<jti>` in Redis with a TTL equal to the access token's remaining lifetime.
- Update auth middleware to reject blacklisted JTIs.
- Decide and test Redis failure behavior. For this service, fail closed on protected requests and return `503` rather than accepting a possibly revoked token.

### Acceptance criteria

- Logout revokes both the refresh token and current access token.
- The blacklisted access token receives `401` after logout.
- The revoked refresh token cannot rotate.
- Redis keys expire no later than the JWT expiration.
- Raw access or refresh tokens are never written to logs or PostgreSQL.

---

## E1-SS-07 — Google OAuth2 and account linking

**Owner:** A  
**Priority:** P1  
**Estimate:** 8 hours  
**Dependencies:** E1-SS-02  
**Merge position:** 7

### Work

- Add Google OAuth configuration and required environment variables.
- Implement the authorization redirect and callback.
- Generate, persist, and validate an OAuth `state` value to prevent CSRF.
- Exchange the authorization code, fetch the verified email/profile, and link or create the user.
- Define safe account-linking behavior when a local account already uses the email.
- Issue the same token pair used by local login.
- Do not place long-lived tokens in redirect query parameters. Return JSON for API clients or exchange a short-lived one-time code at the frontend.

### Acceptance criteria

- Missing, invalid, reused, or expired state is rejected.
- Unverified Google email is rejected.
- Existing accounts are not silently taken over solely because an email string matches.
- OAuth-only users do not require a password hash.
- Provider calls are mocked in automated tests.

---

## E1-SS-08 — Kafka contract and local infrastructure

**Owner:** B  
**Priority:** P0  
**Estimate:** 6 hours  
**Dependencies:** E1-SS-01  
**Merge position:** 8; contract can be prepared earlier, but merge after stable domain models

### Work

- Define Protobuf messages for `user.preference.updated` and `user.registered`.
- Include `event_id`, `event_type`, `schema_version`, `occurred_at`, and `user_id` in event metadata.
- Add `segmentio/kafka-go` behind a small `Publisher` interface.
- Add a single local Kafka broker and health check to Docker Compose.
- Add topic initialization for development.
- Provide a no-op or mock publisher for unit tests.

### Acceptance criteria

- Protobuf generation is reproducible through a Make target.
- The service can connect to local Kafka after its health check passes.
- Unit tests do not require Kafka.
- Events have enough metadata for deduplication and schema evolution.

---

## E1-SS-09 — Publish registration and preference events

**Owner:** B  
**Priority:** P0  
**Estimate:** 8 hours  
**Dependencies:** E1-SS-05 and E1-SS-08  
**Merge position:** 9

### Work

- Publish `user.registered` after successful registration.
- Publish `user.preference.updated` after onboarding, like, unlike, follow, unfollow, and language changes.
- Keep event creation separate from Kafka transport.
- Use a bounded asynchronous publisher with shutdown draining and error metrics. Do not start an unbounded goroutine per request.
- Document delivery semantics. If database/event atomicity is required, create an outbox follow-up or implement a transactional outbox now.

### Acceptance criteria

- Every successful mutation emits exactly the documented event type.
- Failed database mutations emit no event.
- Publisher queue saturation is observable and does not leak goroutines.
- Unit tests assert event payloads using a fake publisher.
- Integration test proves a like event reaches the local topic.

---

## E1-SS-10 — Internal gRPC profile API

**Owner:** A  
**Priority:** P0  
**Estimate:** 8 hours  
**Dependencies:** E1-SS-03 and E1-SS-05  
**Merge position:** 10

### Work

- Define `IdentityService.GetListenerProfile` in `proto/identity.proto`.
- Generate Go client and server code reproducibly.
- Implement the server by calling the same profile service used by HTTP handlers.
- Run gRPC on port `50051` alongside HTTP.
- Add graceful shutdown for both servers.
- Restrict the port to the internal Docker network; document that production should use service identity or mTLS.

### Acceptance criteria

- Valid user ID returns tier, genre seeds, language preferences, followed artists, and liked-song count.
- Invalid UUID returns `InvalidArgument`.
- Missing user returns `NotFound`.
- A test gRPC client verifies the response.
- HTTP and gRPC servers shut down cleanly.

---

## E1-SS-11 — Metrics, structured logging, and health

**Owner:** A  
**Priority:** P1  
**Estimate:** 8 hours  
**Dependencies:** E1-SS-09 and E1-SS-10  
**Merge position:** 11

### Work

- Add request-ID middleware and return the ID in a response header.
- Add structured JSON logging with zap.
- Log request ID, user ID when available, method, route, status, and latency.
- Add Prometheus request count and latency metrics.
- Export DB pool statistics and Kafka publish failure/queue metrics.
- Expose `/metrics`, `/health/live`, and `/health/ready`.
- Readiness must check critical dependencies with short timeouts.

### Acceptance criteria

- No credentials, JWTs, passwords, OAuth codes, or refresh tokens appear in logs.
- Metrics labels do not contain raw URLs, user IDs, or other unbounded values.
- Readiness fails when PostgreSQL is unavailable.
- One HTTP request produces one structured completion log with a request ID.

---

## E1-SS-12 — Documentation and API artifacts

**Owner:** B  
**Priority:** P1  
**Estimate:** 8 hours  
**Dependencies:** Stable HTTP and event contracts from E1-SS-05, E1-SS-07, E1-SS-09, and E1-SS-10  
**Merge position:** 12

### Work

- Replace the current setup notes with a concise local-development README.
- Document all environment variables without committing real secrets.
- Add OpenAPI 3.0 definitions for every HTTP endpoint and error response.
- Update the Postman collection.
- Add a Mermaid architecture diagram covering HTTP, gRPC, PostgreSQL, Redis, and Kafka.
- Document migrations, tests, Docker startup, Protobuf generation, and smoke-test commands.

### Acceptance criteria

- A new developer can start the stack using only the README.
- OpenAPI paths and schemas match handler behavior.
- Postman includes register, login, refresh, logout, OAuth, profile, onboarding, likes, and follows.
- `.env.example` contains placeholders only; `.env` is ignored by Git.

---

## E1-SS-13 — End-to-end test, CI, and release

**Owner:** A as release captain; B pairs on failures  
**Priority:** P0  
**Estimate:** 10 hours across Days 6-7  
**Dependencies:** E1-SS-01 through E1-SS-12  
**Merge position:** 13, always last

### Work

- Add the end-to-end flow:
  - migrate database
  - register
  - login
  - read and patch profile
  - onboarding
  - like and follow
  - verify Kafka event
  - call gRPC profile API
  - logout
  - verify access-token and refresh-token rejection
- Run tests and race detection in CI.
- Build the Docker image in CI.
- Validate migration up and down against PostgreSQL 16.
- Run `go vet ./...` and the configured linter.
- Fix only Epic 1 release blockers.
- Create `v0.1.0` after the default branch is green.

### Acceptance criteria

- CI is green from a clean checkout.
- No test relies on a developer's existing local database state.
- Docker Compose reaches healthy status.
- The full smoke test passes twice consecutively.
- Tag `v0.1.0` points to the exact green release commit.

---

# Seven-Day Timeline

| Day | Developer A | Developer B | Required merges by end of day |
|---|---|---|---|
| **Day 1 — Contracts and safety** | E1-SS-02 auth tests and hardening | E1-SS-01 schema and domain contracts | Merge **01**, then **02** |
| **Day 2 — Core user APIs** | Start E1-SS-04 tier claims; review profile contracts | E1-SS-03 full profile GET/PATCH | Merge **03**, then **04** |
| **Day 3 — User behavior** | E1-SS-06 logout and Redis revocation | E1-SS-05 onboarding, likes, follows, pagination | Merge **05**, then **06** |
| **Day 4 — External integrations** | E1-SS-07 Google OAuth2 | E1-SS-08 Kafka schema and infrastructure | Merge **07**, then **08** |
| **Day 5 — Service interfaces** | E1-SS-10 gRPC API | E1-SS-09 mutation event publishing | Merge **09** before final gRPC integration; then **10** |
| **Day 6 — Operability and docs** | E1-SS-11 metrics/logging; start E1-SS-13 | E1-SS-12 README/OpenAPI/Postman; support integration tests | Merge **11**, then **12** |
| **Day 7 — Release day** | Release captain: E1-SS-13, shared-file reconciliation, release candidate | Pair on E2E failures, Docker/Kafka verification, documentation corrections | Merge **13** last; tag only after CI passes |

---

# Merge Train

Use short-lived branches and merge small PRs continuously. Do not wait until Day 7 to combine both developers' work.

```text
1.  E1-SS-01  Preferences migration + contracts
2.  E1-SS-02  Auth hardening + tests
3.  E1-SS-03  Full profile GET/PATCH
4.  E1-SS-04  Tier claims + middleware
5.  E1-SS-05  Onboarding + likes/follows
6.  E1-SS-06  Logout + Redis revocation
7.  E1-SS-07  Google OAuth2
8.  E1-SS-08  Kafka contract + infrastructure
9.  E1-SS-09  Event publishing
10. E1-SS-10  gRPC API
11. E1-SS-11  Metrics + logging
12. E1-SS-12  Documentation + API artifacts
13. E1-SS-13  E2E + CI + release
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
| Google credentials or consent screen are unavailable | Complete provider-mocked tests and configuration; mark only the live Google smoke test blocked |
| Kafka setup consumes too much time | Preserve the publisher interface and fake tests; use one supported local broker configuration, not custom orchestration |
| Array pagination becomes unreliable | Use normalized liked/followed tables before building the pagination API |
| Both developers modify server wiring | B exposes constructors; A performs all final route and server wiring |
| Event is lost after DB commit | Prefer a transactional outbox; at minimum document at-most-once risk and create a P0 follow-up before production |
| Day 7 becomes a large merge exercise | Enforce the daily merge train and keep Day 7 for verification only |
| Scope exceeds seven-day capacity | Cut architecture-diagram polish first, then live OAuth smoke testing; do not cut auth, migration, rotation, or E2E correctness tests |

---

# Success Metrics

- 13 tickets merged in dependency order
- Zero known P0 defects
- All protected routes reject missing, expired, tampered, logged-out, or under-tier tokens correctly
- Full end-to-end flow passes from a clean environment
- HTTP, Kafka, and gRPC contracts are documented and tested
- Release `v0.1.0` is reproducible from the tagged commit
