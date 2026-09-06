# Sprint 3 - Epic 3 and Epic 4 Completion Plan

**Sprint goal:** Complete the production core of Epic 3 (Playback Event Collection Pipeline) and Epic 4 (Real-Time Event Streaming Platform), after a bounded spillover-resolution gate.

**Team:** Developer A and Developer B  
**Sprint length:** 7 working days  
**Capacity assumption:** 7 focused hours per developer per day = 98 hours  
**Planned work:** 88 hours  
**Protected integration/recovery buffer:** 10 hours  
**Release target:** `v0.3.0` only after the clean-environment end-to-end gate passes twice

> This is a Sprint 3 plan, not a replacement for the Epic documents. Epic 3 and Epic 4 are completed to the explicit boundaries below. Unfinished Epic 2 CRUD, browse, gRPC, event, observability, bulk-import, and documentation work remains visible backlog and does not silently consume Sprint 3 capacity.

---

## Verified Starting Point - 2026-09-05

The baseline below is based on `origin/main`, current remote branches, GitHub PR state, and repository tags.

| Area | Verified state | Sprint 3 treatment |
| --- | --- | --- |
| Epic 1 implementation | Metrics/health, playlists, playlist events, logout, documentation, and supporting identity work are merged to `main`. | Do not reopen implementation. Record the missing release tag as process debt. |
| Epic 1 release | No `v0.1.0` tag exists. | Do not manufacture a historical release during Sprint 3. Track separately unless release ownership explicitly approves it. |
| Epic 2 scaffold | PR #30 is open and contains E2-SS-02 completion. PR #29 overlaps the scaffold and is stale/superseded in practice. | Resolve in the Day 1 spillover gate: merge #30 after green checks and close #29 as superseded if its unique diff is empty. |
| Epic 2 remaining scope | Catalog schema, artist/album/song CRUD, browse, catalog events, gRPC, E2E/release, bulk import, full observability, and full docs are not merged. | Keep in the Epic 2 backlog. They are not prerequisites for storing playback IDs or transporting opaque event identifiers. |
| Epic 3 | No playback service, schema, ingestion API, or playback event producer exists. | Build and release the bounded Epic 3 core in this sprint. |
| Epic 4 | Kafka local infrastructure and identity producers exist. No reusable consumer runtime, retry/DLQ path, replay operation, or streaming worker exists. | Build and release the bounded Epic 4 core in this sprint. |

### Completion count at sprint start

- Epic 1: implementation substantially landed; release/tag evidence incomplete.
- Epic 2 core plan: E2-SS-02 implemented in open PR #30; the other planned catalog capabilities are not merged.
- Epic 3: 0 sprint stories complete.
- Epic 4: 0 sprint stories complete; Kafka bootstrap and producer patterns are reusable foundations, not Epic 4 completion.

---

## Completion Boundary

### Epic 3 is complete for this sprint when

- An authenticated playback service accepts single and batched telemetry for `play`, `pause`, `seek`, `skip`, `replay`, and `complete`.
- Each request carries a client-generated idempotency/event ID and is stored once in an append-only PostgreSQL ledger.
- User ID comes from the verified access token, never from a trusted request-body field.
- Song IDs remain cross-service UUID references; the playback database does not create invalid cross-database foreign keys.
- Session IDs, event timestamps, playback position, duration, device type, and bounded context metadata are captured.
- Successful commits enqueue versioned playback events transactionally and publish them to Kafka through an outbox relay.
- Liveness, readiness, metrics, structured logs, tests, and service documentation exist.

### Epic 4 is complete for this sprint when

- Kafka has explicit versioned topic contracts for playback telemetry, user interaction inputs, recommendation feedback, retries, and dead letters.
- A streaming worker consumes with a stable consumer group and at-least-once delivery semantics.
- Processing is idempotent, offsets are committed only after successful handling, and SIGTERM drains in-flight work.
- Transient failures use bounded exponential retry; poison messages reach a DLQ with failure metadata and no secrets.
- Operators can request bounded replay from retained Kafka offsets/timestamps without resetting unrelated consumer groups.
- Lag, throughput, retry, DLQ, and processing-latency metrics are exposed.
- Clean-environment E2E demonstrates ingest -> outbox -> Kafka -> consumer -> retry/DLQ/replay with no silent loss.

### Explicitly out of scope

- Completing Epic 2 artist, album, song, browse, gRPC, bulk-import, observability, or documentation stories.
- Recommendation models, feature engineering, ranking, or downstream ML business logic.
- A hosted schema-registry product, multi-region Kafka, exactly-once claims, or production autoscaling.
- UI playback controls and audio streaming itself.
- Arbitrary event mutation or deletion; the telemetry ledger is append-only.

---

## Architecture Decisions Fixed Before Coding

1. Delivery guarantee is **at least once**. Producers and consumers must be idempotent; the plan makes no exactly-once claim.
2. `client_event_id` is a UUID generated by the client and is unique per user. Duplicate submissions return the original accepted result without producing a second Kafka event.
3. Playback PostgreSQL stores user and song IDs as UUID references without cross-database foreign keys.
4. The canonical Kafka key is `user_id` so one user's playback sequence remains ordered within a partition.
5. Event time and ingest time are separate fields. Reject timestamps more than 24 hours in the future; retain late events and mark their lateness.
6. Protobuf envelopes contain `event_id`, `event_type`, `schema_version`, `occurred_at`, `produced_at`, `producer`, and trace/request ID.
7. Offset commit happens after successful processing or successful DLQ publication, never before.
8. Replay uses a separate replay consumer group and bounded offsets/timestamps. It never rewinds the live group.
9. Logs contain IDs and failure classes, not bearer tokens or unbounded/raw context payloads.
10. Shared behavior is copied only when the existing services have already proven it; no new shared-library extraction is part of this sprint.

---

# Gate 0 - Sprint 2 Spillover Resolution

## S3-GATE-01 - Resolve catalog scaffold PR overlap

**Owners:** A and B pair  
**Priority:** P0  
**Estimate:** 4 hours total  
**Dependencies:** None  
**Merge position:** 1

Compare PR #29 and PR #30 against current `main`. Treat PR #30 as the E2-SS-02 completion candidate. Close PR #29 as superseded only after proving it has no required unique behavior. Rebase/update #30, run its acceptance suite, merge it, and record any residual Epic 2 work in backlog.

**Acceptance:**

- [ ] PR #29's unique commits/files are reviewed; any required content is preserved before closure.
- [ ] PR #30 is green for `go vet ./...`, `go test -race ./...`, Compose config, container build, liveness/readiness, and missing/incorrect admin-key `401` behavior.
- [ ] Exactly one catalog scaffold implementation lands on `main`.
- [ ] Epic 2 unfinished stories remain listed as backlog; none are relabeled as complete.
- [ ] Gate consumes no more than four team-hours. If repository permissions block merge, record owner/action and continue Sprint 3 from updated `main` without hiding the blocker.

---

# Epic 3 - Playback Event Collection Pipeline

## E3-SS-01 - Playback service scaffold, schema, and contracts

**Owner:** Developer B  
**Priority:** P0  
**Estimate:** 10 hours  
**Dependencies:** S3-GATE-01  
**Merge position:** 2

Create `music-playback-service` using the proven Go/Chi/pgx service shape. Add `muse_playback`, migrations for append-only `playback_events` and transactional `outbox_events`, health endpoints, graceful shutdown, Dockerfile, `.env.example`, and root Compose wiring. Define transport-independent event/session domain types.

Minimum playback fields: `id`, `client_event_id`, authenticated `user_id`, `song_id`, `session_id`, `event_type`, `occurred_at`, `position_ms`, optional `duration_ms`, `device_type`, bounded `context`, `ingested_at`. Enforce unique `(user_id, client_event_id)` and indexes for `(user_id, occurred_at)` and `(session_id, occurred_at)`.

**Acceptance:**

- [ ] Up/down migrations pass on a fresh `muse_playback` database.
- [ ] No foreign key crosses service databases.
- [ ] Constraints reject negative positions/durations and unsupported event types.
- [ ] `/health/live` is process-only; `/health/ready` fails when PostgreSQL is unavailable.
- [ ] Service boots in Compose with placeholder development values.
- [ ] `go vet ./...` and `go test -race ./...` pass.

## E3-SS-02 - Authenticated single and batch ingestion

**Owner:** Developer A  
**Priority:** P0  
**Estimate:** 8 hours  
**Dependencies:** E3-SS-01  
**Merge position:** 3

Implement `POST /v1/playback/events` and `POST /v1/playback/events:batch`. Validate bearer JWTs using the established identity claim contract. Derive `user_id` from the token. Validate UUIDs, event types, timestamps, position/duration relationships, batch size, and context size.

Batch behavior is all-or-nothing for malformed requests, while duplicate `client_event_id` entries are idempotent successes. Cap batches at 100 events and request context at 4 KiB per event.

**Acceptance:**

- [ ] Missing/invalid/expired token returns `401`.
- [ ] Body user IDs cannot override authenticated identity.
- [ ] Valid single event returns `202 Accepted` with event ID.
- [ ] Valid batch returns per-event accepted/duplicate status in input order.
- [ ] Invalid type, UUID, timestamp, size, or position returns structured `400`.
- [ ] Retrying the same client event creates one ledger row.
- [ ] Table-driven handler/service tests pass under `-race`.

## E3-SS-03 - Session semantics and telemetry integrity

**Owner:** Developer A  
**Priority:** P0  
**Estimate:** 7 hours  
**Dependencies:** E3-SS-02  
**Merge position:** 4

Add service-level session validation without turning the collector into a playback state machine. Preserve events as facts while deriving quality flags for impossible ordering, completion percentage, lateness, and session duration. Expose authenticated `GET /v1/playback/sessions/{sessionID}/events` for operational verification with cursor pagination.

**Acceptance:**

- [ ] Events are returned in deterministic `(occurred_at, id)` order.
- [ ] Session query is scoped to the authenticated user.
- [ ] Completion percentage is bounded to `[0, 1]`.
- [ ] Out-of-order and late events are retained and flagged, not silently dropped.
- [ ] Invalid session ownership returns `404` rather than leaking another user's session.
- [ ] Cursor and pagination tests cover empty, first, middle, and final pages.

## E3-SS-04 - Transactional outbox and playback event publication

**Owner:** Developer B  
**Priority:** P0  
**Estimate:** 9 hours  
**Dependencies:** E3-SS-01, E3-SS-02, E4-SS-01  
**Merge position:** 6

In one database transaction, store each accepted telemetry event and enqueue a versioned Protobuf envelope. Publish `playback.event.v1` directly after commit and retain failed messages for a bounded relay. Use `user_id` as the Kafka key. The direct publish path and relay fallback must be independently testable.

**Acceptance:**

- [ ] Ledger row and outbox row commit atomically.
- [ ] Each newly accepted event produces exactly one outbox record.
- [ ] Duplicate client events do not create another outbox record.
- [ ] Direct publish failure does not lose the event; relay later publishes it.
- [ ] Disabling relay does not disable direct publishing.
- [ ] Published payload and headers match the versioned contract.
- [ ] Unit tests use fakes; one integration test proves delivery to Kafka.

## E3-SS-05 - Playback observability, hardening, and docs

**Owner:** Developer A  
**Priority:** P1  
**Estimate:** 5 hours  
**Dependencies:** E3-SS-02, E3-SS-03, E3-SS-04  
**Merge position:** 10

Add request IDs, structured completion logs, bounded-label metrics, body/time limits, and operational documentation. Metrics include accepted/duplicate/rejected events, batch size, ingest latency, outbox backlog, and publish failures.

**Acceptance:**

- [ ] `/metrics` exposes documented Prometheus metrics with bounded labels.
- [ ] One request produces one structured completion log.
- [ ] Tokens and raw context payloads never appear in logs.
- [ ] Server enforces request-body, header, read, write, and idle limits.
- [ ] README documents configuration, migrations, endpoints, event semantics, and local verification.

---

# Epic 4 - Real-Time Event Streaming Platform

## E4-SS-01 - Topic topology and versioned event contracts

**Owner:** Developer B  
**Priority:** P0  
**Estimate:** 6 hours  
**Dependencies:** E3-SS-01 domain contract  
**Merge position:** 5

Define versioned Protobuf envelopes and explicit topics for `playback.event.v1`, existing user interaction inputs, `recommendation.feedback.v1`, retry traffic, and dead letters. Extend `kafka-init` idempotently. Document partition key, retention, compatibility, ownership, producer, and intended consumers for every topic.

**Acceptance:**

- [ ] Protobuf generation is reproducible and generated files are checked in.
- [ ] New fields follow backward-compatible optional/additive rules.
- [ ] Topic creation is idempotent and does not rely on broker auto-create.
- [ ] Local topics have explicit partition and retention configuration.
- [ ] Contract tests assert topic names, headers, keys, and schema version.

## E4-SS-02 - Reusable consumer runtime and offset discipline

**Owner:** Developer A  
**Priority:** P0  
**Estimate:** 9 hours  
**Dependencies:** E4-SS-01, E3-SS-04  
**Merge position:** 7

Create `music-event-streaming-service` with a reusable Kafka consumer loop, handler interface, stable group IDs, manual commit after processing, bounded concurrency, cancellation, health/readiness, and graceful drain. Start with playback telemetry and support existing identity interaction topics through configuration.

**Acceptance:**

- [ ] Messages are committed only after successful handler completion.
- [ ] Restart resumes from committed offsets without silent skips.
- [ ] Duplicate delivery is harmless through an idempotency store keyed by event ID and handler name.
- [ ] SIGTERM stops intake, drains bounded in-flight work, commits successes, and exits before timeout.
- [ ] Readiness fails when Kafka is unreachable.
- [ ] Unit tests cover cancellation, commit ordering, duplicate delivery, and handler failure.

## E4-SS-03 - Bounded retry and dead-letter handling

**Owner:** Developer B  
**Priority:** P0  
**Estimate:** 8 hours  
**Dependencies:** E4-SS-02  
**Merge position:** 8

Classify processing failures as retryable or permanent. Retry transient failures with exponential backoff and jitter using retry topics; after three attempts, or immediately for permanent/schema failures, publish a dead-letter envelope. Commit the source offset only after retry/DLQ publication succeeds.

**Acceptance:**

- [ ] Retry attempts are bounded and encoded in headers.
- [ ] Poison messages cannot block a partition forever.
- [ ] DLQ preserves original topic, partition, offset, key, schema version, failure class, and trace ID.
- [ ] DLQ payload/logs contain no authorization secrets.
- [ ] If retry/DLQ publication fails, the source offset remains uncommitted.
- [ ] Integration test proves success, transient recovery, and poison-message DLQ paths.

## E4-SS-04 - Operator-controlled event replay

**Owner:** Developer A  
**Priority:** P0  
**Estimate:** 7 hours  
**Dependencies:** E4-SS-02, E4-SS-03  
**Merge position:** 9

Add an admin-key-protected replay operation that creates a separate replay consumer group for an allowlisted topic and bounded time/offset range. Require a reason, maximum event count, dry-run preview, audit record, and one active replay per target group/topic.

**Acceptance:**

- [ ] Missing/incorrect `X-Admin-Key` returns `401`.
- [ ] Topic, time range, offsets, and maximum count are validated before execution.
- [ ] Dry run reports the planned range without consuming messages.
- [ ] Replay never rewinds or commits offsets for the live consumer group.
- [ ] Every request and outcome is audit logged without raw payloads.
- [ ] Integration test replays a bounded range and proves the live group is unchanged.

## E4-SS-05 - Multi-topic streaming, feedback readiness, and observability

**Owner:** Developer B  
**Priority:** P1  
**Estimate:** 7 hours  
**Dependencies:** E4-SS-02, E4-SS-03, E4-SS-04  
**Merge position:** 11

Configure the worker for playback, existing user interaction events, and the future `recommendation.feedback.v1` contract. Add consumer lag, processed, duplicate, retry, DLQ, replay, and handler-latency metrics plus structured logs and alert thresholds. This story proves platform behavior; it does not implement recommendation business logic.

**Acceptance:**

- [ ] Adding an allowlisted topic/handler requires configuration and registration, not a second consumer implementation.
- [ ] Unknown schema versions route safely to DLQ.
- [ ] Metrics avoid user ID, event ID, partition, and error-message labels.
- [ ] Logs correlate source topic/partition/offset and trace ID without raw payloads.
- [ ] A mixed-topic integration test proves independent progress when one handler fails.
- [ ] Runbook covers lag, retries, DLQ inspection, replay, and safe shutdown.

---

# Integration and Release

## S3-REL-01 - Epic 3/4 end-to-end gate, CI, and `v0.3.0`

**Owner:** Developer A as release captain; Developer B pairs  
**Priority:** P0  
**Estimate:** 8 hours  
**Dependencies:** E3-SS-01 through E3-SS-05 and E4-SS-01 through E4-SS-05  
**Merge position:** 12

Run the complete flow twice from a clean checkout: start infrastructure, migrate, authenticate, ingest single and batch events, verify idempotency, verify PostgreSQL/outbox state, consume Kafka events, force retry and DLQ, execute bounded replay, inspect metrics, and terminate both services gracefully. Add CI jobs and update root architecture documentation.

**Acceptance:**

- [ ] `go test -race ./...`, `go vet ./...`, formatting, vulnerability checks, and production builds pass in both new modules.
- [ ] All migrations apply and roll back from a clean database.
- [ ] Compose reports PostgreSQL, Kafka, playback service, and streaming worker healthy.
- [ ] E2E passes twice with no missing or duplicate side effects beyond documented at-least-once delivery.
- [ ] Kafka outage/recovery leaves committed telemetry in the outbox and eventually publishes it.
- [ ] Retry, DLQ, and replay paths are demonstrated with recorded commands/results.
- [ ] Root README contains architecture, local startup, contracts, and operational commands.
- [ ] CI is green on the exact release commit.
- [ ] `v0.3.0` is created only after every prior condition is met.

---

# Seven-Day Timeline

| Day | Developer A | Developer B | Required merge/result |
| --- | --- | --- | --- |
| **1** | Pair on S3-GATE-01; begin E3-SS-02 API/auth design | Pair on S3-GATE-01; start E3-SS-01 scaffold/schema | Catalog overlap resolved; E3-SS-01 branch opened |
| **2** | Finish E3-SS-02 single/batch ingestion | Finish E3-SS-01; start E4-SS-01 contracts/topics | Merge E3-SS-01, then E3-SS-02; E4 contract reviewed |
| **3** | E3-SS-03 session integrity/query | Finish E4-SS-01; start E3-SS-04 outbox | Merge E3-SS-03, E4-SS-01 |
| **4** | E4-SS-02 consumer runtime | Finish E3-SS-04 publisher/relay | Merge E3-SS-04, then E4-SS-02 |
| **5** | Finish E4-SS-02; start E4-SS-04 replay | E4-SS-03 retry/DLQ | Merge E4-SS-03; replay integration starts |
| **6** | Finish E4-SS-04; E3-SS-05 hardening | E4-SS-05 multi-topic/observability | Merge E4-SS-04, E3-SS-05, E4-SS-05 |
| **7** | Release captain for S3-REL-01 | Pair on failure drills, docs, and release | E2E twice, CI green, merge release PR, tag `v0.3.0` |

## Daily stop rule

No developer starts the next day's P1 work while a previous-day P0 merge is red or unresolved. Use the 10-hour buffer for P0 repair and integration. If the buffer is exhausted, cut convenience/read-query scope from E3-SS-03 before cutting durability, auth, idempotency, retry, DLQ, replay safety, or release verification.

---

# Merge Train

```text
1.  S3-GATE-01  Resolve/merge catalog scaffold; retire overlap
2.  E3-SS-01    Playback scaffold, schema, contracts
3.  E3-SS-02    Authenticated single/batch ingestion
4.  E3-SS-03    Session semantics and telemetry integrity
5.  E4-SS-01    Topic topology and versioned contracts
6.  E3-SS-04    Transactional outbox and publication
7.  E4-SS-02    Consumer runtime and offset discipline
8.  E4-SS-03    Retry and DLQ
9.  E4-SS-04    Bounded replay
10. E3-SS-05    Playback observability and hardening
11. E4-SS-05    Multi-topic platform and observability
12. S3-REL-01   E2E, CI, and v0.3.0
```

## Merge rules

1. Rebase each ticket branch on the latest `main` before review.
2. Do not merge a ticket before every direct dependency is on `main`.
3. Require non-author review for every P0 ticket.
4. Require `go test -race ./...`, `go vet ./...`, and formatting checks per affected module.
5. Migration PRs must include tested up/down paths; generated Protobuf code stays with its source contract.
6. Root `docker-compose.yml`, topic initialization, module files, and server wiring have named owners in each PR to reduce conflict churn.
7. Never merge two red branches together to make either one pass.
8. Squash only after preserving useful migration/contract history in the PR description.
9. A ticket is not complete because code exists on a branch; it is complete only when merged with evidence.
10. Release tagging is a separate gate after merge and clean-environment verification.

---

# Risk Controls

| Risk | Trigger | Response |
| --- | --- | --- |
| Sprint 2 spillover consumes Sprint 3 | Gate exceeds four team-hours | Record blocker and owner; do not pull Epic 2 feature work into this sprint. Continue from latest stable `main`. |
| Epic 3 depends on unfinished catalog CRUD | Song validation cannot call catalog | Treat song ID as an opaque UUID and validate shape only. Add asynchronous validation later; do not create cross-database FK coupling. |
| Duplicate or out-of-order telemetry | Client retries or mobile reconnects | Unique client event IDs, append-only ledger, user-keyed partitions, event-time fields, and quality flags. |
| Kafka outage loses events | Direct publish fails | Transactional outbox plus relay; readiness reflects required dependencies according to documented degraded behavior. |
| Poison event stalls a partition | Handler repeatedly fails | Bounded retries, classification, DLQ, and commit only after successful handoff. |
| Replay corrupts live offsets | Operator resets active group | Separate replay group, dry run, bounded range, audit record, and admin authorization. |
| Metrics create cardinality/PII problems | IDs or raw errors become labels | Fixed label allowlists; IDs only in structured logs where necessary; no payloads or tokens. |
| Day 7 becomes a feature day | P0 work slips past Day 6 | Use buffer and cut P1 convenience scope; Day 7 remains integration/release only. |

---

# Success Metrics

- S3-GATE-01 ends with one authoritative catalog scaffold path and explicit Epic 2 backlog.
- 10 Epic 3/4 stories merge in dependency order with evidence.
- At least six playback event types are accepted singly and in batches.
- Duplicate client events produce one ledger/outbox record.
- A forced Kafka outage recovers without losing committed telemetry.
- A poison event reaches DLQ after the configured retry bound and does not block later events.
- A bounded replay completes without changing the live consumer group's offsets.
- P95 local ingest latency and consumer lag are recorded as baselines, not invented targets.
- The clean E2E passes twice and `v0.3.0` points to that exact green commit.
