# Sprint 3 - Epic 3 and Epic 4 Learning Outcomes

**Scope:** Playback Event Collection Pipeline plus Real-Time Event Streaming Platform  
**Team:** Developer A and Developer B  
**Release target:** `v0.3.0`

## Sprint Learning Goal

By the end of Sprint 3, the team should be able to design and prove an at-least-once event pipeline end to end: authenticated telemetry ingestion, idempotent persistence, transactional publication, ordered Kafka transport, safe consumer offsets, bounded retries, dead-letter handling, operator-controlled replay, and observable recovery.

The process goal is equally important: after Sprint 2 produced significant spillover, completion must be based on merged evidence and release gates rather than code existing on a branch. Sprint 3 protects integration capacity and prevents unrelated Epic 2 work from being hidden inside the new commitment.

---

## Core Engineering Outcomes

### Epic 3 - Playback collection

- [ ] Explain why client event IDs and a unique `(user_id, client_event_id)` constraint are required for safe mobile/client retries.
- [ ] Distinguish event time from ingest time and preserve late/out-of-order telemetry without silently corrupting sequence data.
- [ ] Derive user identity from verified authentication claims rather than trusting request payloads.
- [ ] Design a bounded batch API with deterministic per-event results.
- [ ] Keep service-owned databases independent by avoiding cross-database foreign keys.
- [ ] Implement an atomic ledger/outbox transaction and explain which failure window it closes.
- [ ] Demonstrate that direct Kafka publication and relay fallback are separate paths.

### Epic 4 - Streaming platform

- [ ] Explain at-least-once delivery and why idempotent consumers are required.
- [ ] Demonstrate correct offset ordering: process first, then commit.
- [ ] Implement graceful drain without accepting unbounded new work during shutdown.
- [ ] Classify retryable and permanent failures and prevent poison messages from blocking a partition.
- [ ] Preserve enough source metadata in a DLQ envelope for diagnosis and safe reprocessing.
- [ ] Replay a bounded range with a separate group without altering live consumer offsets.
- [ ] Design bounded-cardinality metrics for throughput, lag, retries, DLQ, replay, and latency.
- [ ] Evolve Protobuf contracts additively and reject unknown/incompatible versions safely.

### Delivery discipline

- [ ] Distinguish merged, open, superseded, and unimplemented work during sprint planning.
- [ ] Keep a spillover gate time-boxed instead of allowing prior-sprint scope to consume the new sprint silently.
- [ ] Preserve Day 7 for integration/release rather than late feature development.
- [ ] Record actual commands and results for migrations, race tests, Kafka failure drills, and E2E runs.
- [ ] Tag only the exact commit that passed the release gate.

---

## Daily Learning Tracker

| Day | Developer A demonstrates | Developer B demonstrates | Required evidence |
| --- | --- | --- | --- |
| **1** | Contract-first ingestion design and evidence-based spillover triage | Service/database boundaries and migration design | Catalog overlap disposition; schema review notes |
| **2** | JWT-derived identity, validation, and idempotent batch behavior | Versioned Protobuf and explicit Kafka topic ownership | Ingestion tests; migration up/down; generated contract |
| **3** | Event-time/session integrity and cursor ordering | Atomic ledger/outbox transaction | Session tests; transaction failure tests |
| **4** | Manual offset commit, idempotent handling, graceful drain | Direct publish plus relay recovery | Real Kafka handoff and shutdown evidence |
| **5** | Safe bounded replay with isolated group IDs | Retry classification, backoff, and DLQ semantics | Offset snapshots; retry/DLQ integration results |
| **6** | Secure logs, limits, playback metrics | Multi-topic failure isolation and streaming metrics | Metrics samples; security review; runbook |
| **7** | Release leadership and clean-environment verification | Failure drills, documentation, and release pairing | Two E2E runs, green CI, verified tag |

---

## Required Technical Explanations

Each developer should be able to answer these without reading the implementation:

1. What happens if PostgreSQL commits but Kafka is unavailable?
2. What happens if a consumer completes its side effect and crashes before committing the offset?
3. Why is committing before processing unsafe?
4. How does a duplicate client request avoid producing a duplicate outbox event?
5. Why is `user_id` the Kafka partition key for playback telemetry?
6. What is the difference between a retry topic, DLQ, and replay consumer group?
7. Why must replay not reset the live consumer group?
8. Which fields are safe metric labels, and which would create cardinality or privacy problems?
9. How do liveness and readiness differ for the playback and streaming services?
10. Which evidence proves Epic 3/4 completion beyond a successful build?

---

## Individual Reflection

### Developer A

- [ ] I can trace one request from JWT validation through persistence, outbox publication, consumption, retry, and offset commit.
- [ ] I can explain and demonstrate graceful consumer shutdown.
- [ ] I can run a replay dry run and prove that live offsets remain unchanged.

**Most important concept learned:**

**Most difficult failure mode tested:**

**Decision I would change next time:**

**Topic requiring more practice:**

### Developer B

- [ ] I can explain the playback schema constraints and why cross-service IDs do not use local foreign keys.
- [ ] I can evolve the Protobuf envelope without breaking existing consumers.
- [ ] I can force direct publish failure and prove eventual relay delivery.
- [ ] I can explain why a DLQ publish must succeed before the source offset is committed.

**Most important concept learned:**

**Most difficult failure mode tested:**

**Decision I would change next time:**

**Topic requiring more practice:**

---

## Completion Evidence

- [ ] Spillover gate result records PR #29/#30 disposition and remaining Epic 2 backlog.
- [ ] Links to every merged Sprint 3 PR are recorded in `Sprint3_Tracker.md`.
- [ ] Playback and streaming unit/integration/race test commands and outputs are recorded.
- [ ] Migration up/down evidence is recorded for a clean `muse_playback` database.
- [ ] Kafka topic list, consumer group offsets, retry attempts, and DLQ evidence are recorded.
- [ ] Replay evidence includes live-group offsets before and after.
- [ ] Kafka outage and recovery prove no loss of committed telemetry.
- [ ] Metrics samples demonstrate bounded labels.
- [ ] Both clean E2E runs and CI links are recorded.
- [ ] `v0.3.0` points to the exact verified commit.

## Final Learning Outcome

Sprint 3 succeeds when the team can explain and demonstrate the pipeline's failure behavior, not merely its happy path. A playback request must be traceable through durable storage and streaming, retries must terminate safely, replay must be isolated from live processing, and every completion claim must have merged and repeatable evidence.
