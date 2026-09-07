# Sprint 3 - Epic 3 and Epic 4 Completion Tracker

**Sprint goal:** Deliver the playback telemetry collection pipeline and the real-time Kafka processing platform, including durable publication, retry/DLQ, bounded replay, observability, and release evidence.

**Dates:** Record at kickoff  
**Team:** Developer A and Developer B  
**Duration:** 7 working days  
**Capacity:** 98 hours total; 88 planned; 10 protected for integration/recovery  
**Release target:** `v0.3.0`

The detailed contracts, estimates, and acceptance criteria are in [Sprint3_TODOS.md](Sprint3_TODOS.md). This tracker records execution evidence only.

---

## Starting Baseline

| Scope | Complete and merged | Open/in review | Not complete |
| --- | --- | --- | --- |
| Epic 1 | Implementation through playlists and playlist events | None identified | Historical `v0.1.0` release evidence/tag |
| Epic 2 | Catalog scaffold merged through PR #29 and hardened by PR #30 | None | Remaining catalog schema/features, events, gRPC, docs, E2E/release |
| Epic 3 | None | None | Entire bounded Sprint 3 scope |
| Epic 4 | Kafka bootstrap and identity producers are reusable foundations | None | Consumer runtime, retry/DLQ, replay, platform observability |

**Baseline rule:** update this table only with merged PR/tag evidence. Branch-local code is not complete.

---

## Status Legend

- `[ ]` Not started
- `[~]` In progress
- `[R]` In review
- `[x]` Complete and merged with evidence
- `[!]` Blocked; owner, cause, and next action recorded
- `[C]` Cut from sprint with an explicit reason

---

## Sprint Board

| Status | ID | Story | Owner | Estimate | Dependencies | PR | Evidence |
| --- | --- | --- | --- | ---: | --- | --- | --- |
| [x] | S3-GATE-01 | Resolve catalog scaffold overlap | A+B | 4h | None | [#29](https://github.com/jeelan-ds786/music-recommender-system/pull/29), [#30](https://github.com/jeelan-ds786/music-recommender-system/pull/30) | Both merged in order; local acceptance suite passed on `main` at `81d620b`. |
| [~] | E3-SS-01 | Playback scaffold, schema, contracts | B | 10h | S3-GATE-01 | Pending | Implemented locally; all acceptance checks pass on `main` plus the working tree. Awaiting PR review and merge. |
| [ ] | E3-SS-02 | Authenticated single/batch ingestion | A | 8h | E3-SS-01 | | |
| [ ] | E3-SS-03 | Session semantics and integrity | A | 7h | E3-SS-02 | | |
| [ ] | E4-SS-01 | Topic topology and versioned contracts | B | 6h | E3-SS-01 | | |
| [ ] | E3-SS-04 | Transactional outbox and publication | B | 9h | E3-SS-01, E3-SS-02, E4-SS-01 | | |
| [ ] | E4-SS-02 | Consumer runtime and offsets | A | 9h | E4-SS-01, E3-SS-04 | | |
| [ ] | E4-SS-03 | Retry and dead-letter handling | B | 8h | E4-SS-02 | | |
| [ ] | E4-SS-04 | Operator-controlled replay | A | 7h | E4-SS-02, E4-SS-03 | | |
| [ ] | E3-SS-05 | Playback observability/hardening | A | 5h | E3-SS-02, E3-SS-03, E3-SS-04 | | |
| [ ] | E4-SS-05 | Multi-topic platform/observability | B | 7h | E4-SS-02, E4-SS-03, E4-SS-04 | | |
| [ ] | S3-REL-01 | E2E, CI, docs, `v0.3.0` | A+B | 8h | All above | | |

---

## Daily Execution Tracker

### Day 1 - Spillover gate and playback foundation

**Required result:** catalog overlap has one disposition; playback schema/scaffold PR is open.

- [x] Compare PR #29 and PR #30 against current `main`.
- [x] Preserve unique behavior and record the authoritative merged sequence.
- [x] Record unresolved Epic 2 stories in backlog.
- [x] B creates playback module, database, migrations, and domain contracts.
- [ ] A writes ingestion/auth contract tests against agreed DTOs.
- [x] Confirm the reviewed schema contract contains no cross-database foreign keys.

**End-of-day evidence:**

- PR(s): [#29](https://github.com/jeelan-ds786/music-recommender-system/pull/29) merged first as commit `985d2ed`; [#30](https://github.com/jeelan-ds786/music-recommender-system/pull/30) merged next as commit `8534e2d`.
- Tests/commands: `git log --left-right --cherry-pick origin/pr-30...origin/pr-29`; merged-delta review; `go vet ./...`; `go test -race ./...`; `docker compose config --quiet`; `docker build -q music-catalog-service`.
- Actual result: PR #29 supplied the initial catalog service and PR #30 retained it while adding the E2-SS-02 hardening delta. Both PR check suites were green; all local acceptance commands passed on `main` at `81d620b`. Exactly one final `music-catalog-service` implementation exists.
- Blocker and owner: Playback scaffold, migrations, and executable contract tests are not started; Developer A and Developer B retain their assigned Day 1 work.
- Handoff: Implement the reviewed contract below in E3-SS-01, then write E3-SS-02 handler/service tests against those types before endpoint logic.
- Buffer consumed: Record at end of Day 1; the evidence pass itself did not require recovery work.

**E3-SS-01 implementation evidence:**

- Files: `music-playback-service` module, playback/outbox migrations, health endpoints, Docker image, environment example, documentation, root Compose services, and dedicated CI jobs.
- Migration result: migrations 1 and 2 applied, rolled back, and reapplied on a fresh `muse_playback` database. PostgreSQL reported zero foreign keys on `playback_events`; required unique and query indexes were present.
- Constraint result: PostgreSQL rejected unsupported event types, negative positions, negative durations, and ledger updates. The append-only trigger passed its direct database check.
- Health result: with PostgreSQL available, liveness/readiness returned `200`; after stopping PostgreSQL, liveness remained `200` and readiness returned `503` with `postgres` identified as failed.
- Runtime result: Compose built and booted the service on port 8082 as non-root user `playback`; Docker SIGTERM produced exit code 0; the stack was restored healthy afterward.
- Quality result: `gofmt`, `go mod tidy -diff`, `go mod verify`, `go vet ./...`, `go build ./cmd/server`, and `go test -race ./...` passed. `docker compose config --quiet` passed. Editor diagnostics reported no errors.
- Remaining gate: open a PR, obtain review, run GitHub CI, and merge before changing E3-SS-01 to `[x]` or incrementing the merged-story count.

#### Day 1 ingestion and schema contract review

| Boundary | Reviewed contract | Verification target |
| --- | --- | --- |
| Authentication | The request has no trusted `user_id`; middleware supplies the verified JWT subject to the application service. | A body field cannot override identity; missing, invalid, or expired JWT returns `401`. |
| Single ingestion | `POST /v1/playback/events` accepts a client UUID, song UUID, session UUID, event type, event timestamp, position, optional duration, device type, and bounded context. | A valid new event returns `202` and its server event ID; retry returns the existing result. |
| Batch ingestion | `POST /v1/playback/events:batch` accepts at most 100 events and returns one deterministic result per input in input order. | Malformed requests fail atomically; valid duplicates are idempotent successes. |
| Event identity | `client_event_id` is client-generated; `(user_id, client_event_id)` is unique. The server owns the ledger event ID. | Concurrent retries create one ledger row and one outbox record. |
| Time semantics | `occurred_at` is client event time and `ingested_at` is server time. Events over 24 hours in the future are rejected; late events are retained. | Tests distinguish future rejection from accepted late/out-of-order telemetry. |
| Value constraints | Event type is one of `play`, `pause`, `seek`, `skip`, `replay`, or `complete`; position and duration are non-negative; context is at most 4 KiB. | Database constraints backstop transport validation. |
| Service ownership | `user_id` and `song_id` are opaque UUID references. Playback owns `playback_events` and `outbox_events`; no foreign key crosses service databases. | Fresh migration up/down succeeds without identity or catalog database access. |
| Transaction boundary | A newly accepted ledger row and its versioned outbox event commit atomically. Duplicate acceptance creates neither a second ledger row nor a second outbox event. | Transaction-failure and duplicate-concurrency tests prove the invariant. |

**Schema review disposition:** approved for E3-SS-01 implementation. Required indexes are unique `(user_id, client_event_id)`, `(user_id, occurred_at)`, and `(session_id, occurred_at)`. The append-only ledger and transactional outbox remain playback-owned.

### Day 2 - Ingestion and Kafka contracts

**Required result:** E3-SS-01 and E3-SS-02 merged; E4 event contracts reviewed.

- [ ] A completes authenticated single and batch endpoints.
- [ ] B completes migration integration and E4-SS-01 Protobuf/topic contracts.
- [ ] Verify duplicate client event IDs are idempotent.
- [ ] Verify body user IDs cannot override JWT identity.
- [ ] Run migration up/down and race tests before merge.

**End-of-day evidence:**

- PR(s):
- Tests/commands:
- Actual result:
- Blocker and owner:
- Handoff:
- Buffer consumed:

### Day 3 - Session integrity and durable publication

**Required result:** session semantics and topic contracts merged; outbox path executable.

- [ ] A implements deterministic session query and quality flags.
- [ ] B completes E4-SS-01 and implements atomic ledger/outbox transaction.
- [ ] Confirm event-time vs ingest-time behavior.
- [ ] Confirm direct-publish failure leaves a relayable outbox record.

**End-of-day evidence:**

- PR(s):
- Tests/commands:
- Actual result:
- Blocker and owner:
- Handoff:
- Buffer consumed:

### Day 4 - Producer/consumer handoff

**Required result:** E3-SS-04 and E4-SS-02 merged in that order.

- [ ] B finishes direct publisher and fallback relay tests.
- [ ] A implements manual-commit consumer runtime and idempotency store.
- [ ] Run real Kafka producer-to-consumer integration test.
- [ ] Send SIGTERM during in-flight processing and verify graceful drain.

**End-of-day evidence:**

- PR(s):
- Tests/commands:
- Actual result:
- Blocker and owner:
- Handoff:
- Buffer consumed:

### Day 5 - Failure handling and replay

**Required result:** retry/DLQ merged; bounded replay works in integration.

- [ ] B implements retry classification, backoff/jitter, and DLQ envelope.
- [ ] A implements replay dry-run, bounded execution, separate group, and audit record.
- [ ] Prove source offsets remain uncommitted if retry/DLQ publication fails.
- [ ] Prove replay does not alter live group offsets.

**End-of-day evidence:**

- PR(s):
- Tests/commands:
- Actual result:
- Blocker and owner:
- Handoff:
- Buffer consumed:

### Day 6 - Hardening and platform observability

**Required result:** all feature stories merged; no P0 feature work enters Day 7.

- [ ] A completes playback limits, logs, metrics, and docs.
- [ ] B completes multi-topic configuration, streaming metrics, and runbook.
- [ ] Run mixed-topic failure-isolation test.
- [ ] Run security review for tokens, raw payloads, unbounded labels, and replay authorization.
- [ ] Freeze feature changes after the integration candidate is green.

**End-of-day evidence:**

- PR(s):
- Tests/commands:
- Actual result:
- Blocker and owner:
- Handoff:
- Buffer consumed:

### Day 7 - Clean E2E and release

**Required result:** clean flow passes twice, CI is green, exact commit is tagged `v0.3.0`.

- [ ] Start from clean checkout and empty service databases.
- [ ] Apply all migrations.
- [ ] Start identity, playback, Kafka, and streaming services.
- [ ] Authenticate and submit single/batch playback events.
- [ ] Verify ledger, outbox, Kafka publication, consumption, and idempotency.
- [ ] Force transient retry and poison-message DLQ.
- [ ] Run bounded replay and verify live offsets remain unchanged.
- [ ] Verify metrics, readiness, and graceful shutdown.
- [ ] Repeat the complete flow a second time.
- [ ] Merge release PR only after CI passes.
- [ ] Tag the exact verified commit `v0.3.0`.

**End-of-day evidence:**

- Release PR:
- First E2E result:
- Second E2E result:
- CI URL/result:
- Release commit:
- Tag verification:
- Residual risks/backlog:

---

## Dependency Flow

```mermaid
flowchart LR
    G[S3-GATE-01 Spillover gate] --> E301[E3-SS-01 Scaffold and schema]
    E301 --> E302[E3-SS-02 Ingestion]
    E302 --> E303[E3-SS-03 Session integrity]
    E301 --> E401[E4-SS-01 Contracts and topics]
    E302 --> E304[E3-SS-04 Outbox publisher]
    E401 --> E304
    E304 --> E402[E4-SS-02 Consumer runtime]
    E402 --> E403[E4-SS-03 Retry and DLQ]
    E403 --> E404[E4-SS-04 Replay]
    E303 --> E305[E3-SS-05 Playback hardening]
    E304 --> E305
    E404 --> E405[E4-SS-05 Multi-topic observability]
    E305 --> R[S3-REL-01 E2E and release]
    E405 --> R
```

---

## Quality Gates

### Per PR

- [ ] Direct dependencies are already merged.
- [ ] Non-author review completed for P0 work.
- [ ] `gofmt -l .` returns no files in each changed Go module.
- [ ] `go vet ./...` passes in each changed Go module.
- [ ] `go test -race ./...` passes in each changed Go module.
- [ ] New migrations pass up and down.
- [ ] New contracts include source and generated code.
- [ ] No secret, token, or raw event payload is logged.
- [ ] PR description records commands and actual results.

### Sprint release

- [ ] No unresolved merge conflicts or uncommitted generated files.
- [ ] Compose config validates and all required services become healthy.
- [ ] Production containers build as non-root images.
- [ ] Vulnerability scan reports no reachable known vulnerability introduced by Sprint 3.
- [ ] Kafka outage, retry, DLQ, replay, and graceful shutdown drills pass.
- [ ] Documentation and operational runbook match actual commands.
- [ ] E2E passes twice from clean state.
- [ ] Release tag points to the green commit.

---

## Spillover Register

These items are intentionally visible but not part of Sprint 3 commitment unless the sprint finishes early and the product owner explicitly trades scope.

| Item | Status at kickoff | Owner for next planning decision | Why not in Sprint 3 |
| --- | --- | --- | --- |
| Epic 1 `v0.1.0` historical release/tag | Missing | Release owner | Historical process debt; does not unblock Epic 3/4 engineering. |
| E2-SS-01 catalog schema | Not merged | Epic 2 owner | Epic 3 stores opaque song UUIDs and does not require catalog CRUD. |
| E2-SS-03 through E2-SS-06 | Not merged | Epic 2 owner | Catalog feature work would consume the Epic 3/4 capacity. |
| E2-SS-08 through E2-SS-10 and E2-SS-13 | Not merged | Epic 2 owner | Catalog-specific events/gRPC/release are separate from playback streaming. |
| E2-SS-07, E2-SS-11, E2-SS-12 | Previously deferred | Epic 2 owner | Explicit deferred backlog remains unchanged. |

---

## Sprint Health Metrics

Update daily.

| Metric | Target | Current |
| --- | ---: | ---: |
| Planned hours | 88 | 88 |
| Protected buffer | 10 | 10 |
| P0 stories blocked over 1 day | 0 | 0 |
| Open P0 PRs older than 1 day | 0 | 0 |
| Stories merged | 12 | 1 |
| E2E clean passes | 2 | 0 |
| Untriaged spillover items | 0 | 0 |

## Final Outcome

- Epic 3 completion evidence:
- Epic 4 completion evidence:
- Release/tag:
- Scope cut, if any:
- Buffer used:
- What carries to Sprint 4:
