# Sprint 1 - Week 1 Learning Outcomes

**Epic:** User Identity & Listener Profile Service  
**Duration:** 7 days  
**Team:** Developer A and Developer B  
**Release target:** `v0.1.0`

## Sprint Learning Goal

By the end of this sprint, the team should be able to build, test, document, and release a production-style Go service that combines authentication, profile management, PostgreSQL, Redis, Kafka, gRPC, observability, containers, and CI.

The goal is not only to finish endpoints. Each developer should understand why the security, data, integration, and deployment decisions were made.

## Languages And Formats

- [ ] **Go:** Write handlers, middleware, services, repositories, tests, and graceful-shutdown logic.
- [ ] **PostgreSQL SQL:** Create reversible migrations, constraints, indexes, joins, and transactional mutations.
- [ ] **Protocol Buffers:** Define versioned Kafka events and an internal gRPC contract.
- [ ] **YAML:** Configure Docker Compose, GitHub Actions, and OpenAPI.
- [ ] **JSON:** Design API payloads, structured errors, JWT claims, events, and logs.
- [ ] **Markdown and Mermaid:** Document setup, architecture, sprint progress, and API workflows.
- [ ] **Shell and Make:** Automate migrations, tests, builds, Protobuf generation, and local startup.

## Tools And Libraries

| Tool or library | Learning outcome |
|---|---|
| Go | Build an idiomatic, modular backend service. |
| `chi` | Define REST routes and compose HTTP middleware. |
| `pgx/v5` | Use PostgreSQL pools, parameterized queries, and transactions. |
| `golang-migrate` | Apply and roll back version-controlled database migrations. |
| `golang-jwt/jwt/v5` | Generate and validate signed access tokens with typed claims. |
| `bcrypt` | Hash passwords and reduce login timing differences. |
| `go-playground/validator` | Produce machine-readable request validation errors. |
| `go-redis` | Store short-lived revocation and OAuth state data with TTLs. |
| `golang.org/x/oauth2` | Implement Google's authorization-code flow safely. |
| `segmentio/kafka-go` | Publish user and preference events through an abstraction. |
| `google.golang.org/grpc` | Expose an internal typed profile API. |
| `prometheus/client_golang` | Export counters, latency histograms, and dependency metrics. |
| `uber-go/zap` | Produce structured JSON logs with request context. |
| Docker and Docker Compose | Run the service and its dependencies reproducibly. |
| Postman and OpenAPI 3.0 | Describe and manually verify the HTTP API. |
| GitHub Actions | Validate tests, vetting, migrations, and container builds in CI. |

## Core Engineering Outcomes

### Architecture

- [ ] Explain the handler -> service -> repository flow.
- [ ] Use interfaces to isolate databases, Kafka, and external OAuth providers in tests.
- [ ] Keep HTTP, Kafka, and gRPC transport concerns outside domain logic.
- [ ] Use dependency injection instead of creating dependencies inside business services.

### Authentication And Security

- [ ] Explain access-token and refresh-token responsibilities.
- [ ] Generate 15-minute HS256 access tokens with validated `user_id`, `tier`, `iat`, `exp`, and `jti` claims.
- [ ] Generate refresh tokens with `crypto/rand`, store only SHA-256 hashes, and rotate them atomically.
- [ ] Prevent account enumeration by returning identical login errors and performing a dummy bcrypt comparison.
- [ ] Revoke refresh tokens in PostgreSQL and access-token JTIs in Redis during logout.
- [ ] Protect OAuth2 with a stored, expiring, one-time `state` value.
- [ ] Explain why passwords, raw tokens, OAuth codes, and secrets must never appear in logs.

### Database And Profile Design

- [ ] Write migrations with working up and down paths.
- [ ] Use foreign keys, unique constraints, defaults, and indexes to enforce data integrity.
- [ ] Join user, listener profile, and preference data for `GET /me`.
- [ ] Implement partial `PATCH /me` updates without overwriting omitted fields.
- [ ] Make like and follow mutations atomic and idempotent.
- [ ] Explain why normalized interaction tables provide safer cursor pagination than unordered UUID arrays.

### Events And Service Communication

- [ ] Define versioned Protobuf events with event ID, type, user ID, and timestamp metadata.
- [ ] Publish `user.registered` and `user.preference.updated` events.
- [ ] Use a bounded asynchronous publisher rather than an unlimited goroutine per request.
- [ ] Explain event delivery risks between a database commit and Kafka publish.
- [ ] Implement and test `GetListenerProfile` through gRPC.
- [ ] Run HTTP and gRPC servers together with graceful shutdown.

### Testing And Operations

- [ ] Write table-driven unit tests for authentication and validation behavior.
- [ ] Use fakes or mocks for repositories, Kafka, Redis, OAuth providers, and external services.
- [ ] Test expired, malformed, tampered, revoked, and under-tier tokens.
- [ ] Run integration tests against PostgreSQL, Redis, and Kafka.
- [ ] Run an end-to-end flow from registration through logout.
- [ ] Use `go test -race`, `go vet`, and linting to catch defects before merge.
- [ ] Add health, readiness, metrics, request IDs, and structured completion logs.
- [ ] Build and verify a distroless Docker image.

## Seven-Day Learning Tracker

| Day | Developer A learns and demonstrates | Developer B learns and demonstrates | Evidence |
|---|---|---|---|
| **Day 1** | Auth testing, dummy bcrypt comparison, JWT and refresh-token security tests | SQL migration design, GIN indexes, transport-independent domain contracts | [ ] PRs 01 and 02 merged; focused tests pass |
| **Day 2** | Tier claims, typed context values, authorization middleware | Profile joins, patch DTOs, validation, repository tests | [ ] PRs 03 and 04 merged; profile tests pass |
| **Day 3** | JWT IDs, Redis TTL revocation, logout failure behavior | Onboarding, atomic likes/follows, stable cursor pagination | [ ] PRs 05 and 06 merged; mutation tests pass |
| **Day 4** | OAuth2 code flow, state/CSRF protection, safe account linking | Protobuf event design, Kafka producer abstraction, local broker setup | [ ] PRs 07 and 08 merged; provider tests pass |
| **Day 5** | gRPC service implementation, status codes, shared business logic | Event creation, bounded publishing, payload and integration tests | [ ] PRs 09 and 10 merged; Kafka and gRPC tests pass |
| **Day 6** | Prometheus, zap, request IDs, health checks, CI preparation | README, OpenAPI, Postman, Mermaid architecture documentation | [ ] PRs 11 and 12 merged; docs match behavior |
| **Day 7** | Release integration, CI, migration and Docker verification | E2E pairing, Kafka verification, final documentation corrections | [ ] PR 13 merged; CI green; `v0.1.0` tagged |

## Individual Reflection

### Developer A

- [ ] I can explain the complete authentication lifecycle without reading the code.
- [ ] I can explain how JWT expiry, refresh rotation, logout, and OAuth state protect users.
- [ ] I can start, observe, debug, and safely shut down the service.
- [ ] I reviewed at least one P0 ticket written by Developer B.

**Most important concept learned:**  

**Most difficult problem solved:**  

**Topic requiring more practice:**  

### Developer B

- [ ] I can explain how profile and preference data is stored and updated safely.
- [ ] I can explain idempotency, cursor pagination, and event delivery tradeoffs.
- [ ] I can define and test a Kafka event contract independently of the broker.
- [ ] I reviewed at least one P0 ticket written by Developer A.

**Most important concept learned:**  

**Most difficult problem solved:**  

**Topic requiring more practice:**  

## Completion Evidence

- [ ] Links to all merged pull requests are recorded.
- [ ] Unit, integration, race, and end-to-end test results are recorded.
- [ ] Migration up/down output is recorded.
- [ ] Docker Compose health output is recorded.
- [ ] Example HTTP, Kafka, and gRPC requests are documented.
- [ ] Prometheus metrics and structured log examples are captured.
- [ ] Final CI run is green.
- [ ] Release tag `v0.1.0` points to the verified commit.

## Final Outcome

After completing this sprint, both developers should be able to describe how a production-style identity service connects authentication, authorization, durable storage, caching, asynchronous events, internal RPC, observability, testing, containers, and CI into one coherent system.
