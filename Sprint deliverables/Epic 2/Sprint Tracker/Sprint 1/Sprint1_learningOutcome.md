# Sprint 1 - Epic 2 Learning Outcomes

**Epic:** Music Catalog Management Service
**Duration:** 4 days (core scope; bulk import, full observability, and full docs deferred to Sprint 2)
**Team:** Developer A and Developer B
**Release target:** `v0.2.0`

## Sprint Learning Goal

By the end of this sprint, the team should be able to design, build, and
release the core of a second production-style Go service that shares
infrastructure with an existing service (Postgres, Kafka) without sharing
code, and that exposes data for other services to consume rather than
serving end users directly — and do it under a materially tighter
timeline than Epic 1's, which means learning to identify and defer
lower-priority scope deliberately rather than build everything shallowly.

The goal is not only to finish endpoints. Each developer should
understand why the catalog's data model, authorization model, and event
contract were designed the way they were, how they differ deliberately
from Epic 1's identity service, and why bulk import / full observability
/ full docs were the right three things to cut for time.

## Languages And Formats

- [ ] **Go:** Write handlers, services, repositories, and gRPC servers under time pressure without skipping tests.
- [ ] **PostgreSQL SQL:** Design a multi-table relational schema with real foreign keys, a normalized many-to-many join table, and GIN indexes for array-tag filtering.
- [ ] **Protocol Buffers:** Define a versioned Kafka event and an internal gRPC batch-lookup contract.
- [ ] **YAML:** Extend Docker Compose with a second service and a second database on a shared Postgres container.
- [ ] **JSON:** Design API payloads, structured errors, and events.
- [ ] **Markdown and Mermaid:** Write just enough documentation to unblock a new developer, and explicitly flag what's deferred rather than leaving it undocumented.

## Tools And Libraries

| Tool or library | Learning outcome |
|---|---|
| Go | Build a second idiomatic, modular backend service reusing proven patterns without a shared library. |
| `chi` | Define REST routes and compose HTTP middleware, including a non-JWT authorization scheme. |
| `pgx/v5` | Model a relational catalog: foreign keys, cascades, a join table, GIN indexes, and cursor-friendly sort indexes. |
| `golang-migrate` | Apply and roll back a multi-table schema across four migrations. |
| `segmentio/kafka-go` | Reimplement a proven transactional-outbox publishing pattern in a second service. |
| `google.golang.org/grpc` | Design and implement a batch-lookup RPC for downstream service-to-service calls. |
| Docker and Docker Compose | Extend an existing multi-service stack with a new service and database rather than building one from scratch. |

`prometheus/client_golang`, `uber-go/zap`, full OpenAPI/Postman authoring,
and GitHub Actions extensions for this service arrive with Epic 2 Sprint
2 (E2-SS-11 and E2-SS-12).

## Core Engineering Outcomes

### Architecture

- [ ] Explain why this service copies Epic 1's outbox/publisher pattern instead of importing it as a shared library.
- [ ] Keep HTTP, Kafka, and gRPC transport concerns outside domain logic, same discipline as Epic 1.
- [ ] Explain why catalog writes use a static admin key instead of end-user JWTs, and what that decision trades away.
- [ ] Explain the reasoning behind deferring bulk import, full observability, and full docs to a Sprint 2, and why that's a scoping decision, not a quality shortcut on what did ship.

### Data Modeling

- [ ] Explain why featured artists are a normalized join table but genre/mood tags are array columns.
- [ ] Use foreign keys and `ON DELETE CASCADE`/`ON DELETE SET NULL` deliberately.
- [ ] Explain why a JSONB column can be reserved now for a future epic (audio features) without that epic's schema being designed yet.

### Events And Service Communication

- [ ] Define a versioned Protobuf event with an entity-type/operation envelope general enough for three different entity kinds.
- [ ] Build the outbox-then-direct-publish-with-fallback-relay pattern correctly the first time, using the specific bug Epic 1 found late (a disabled relay silently disabling all publishing) as a design constraint from day one.
- [ ] Implement and test a gRPC batch RPC that partially succeeds rather than failing all-or-nothing.
- [ ] Run HTTP and gRPC servers together with graceful shutdown on a second internal port.

### Working Under Compression

- [ ] Identify which planned scope is safe to defer (bulk import, full metrics, full docs) versus what can't be cut (schema correctness, write-endpoint authorization, event correctness).
- [ ] Run a genuinely compressed integration day (Day 4) as a paired push rather than two siloed tickets.
- [ ] Leave a deferred ticket in a state where someone else could pick it up later with no lost context (see this sprint's TODOS file for how E2-SS-07/11/12 are recorded, not deleted).

## Four-Day Learning Tracker

| Day | Developer A learns and demonstrates | Developer B learns and demonstrates | Evidence |
|---|---|---|---|
| **Day 1** | Service scaffolding for a second service, static-key authorization design | SQL relational schema design, join tables vs. arrays, transport-independent domain contracts | [ ] PRs 01 and 02 merged |
| **Day 2** | Album-artist relationship validation, friendly FK error mapping | Artist CRUD, patch DTOs, pagination, repository tests | [ ] PRs 03 and 04 merged |
| **Day 3** | Cursor pagination reused across a second dataset shape, filter composition | Song CRUD, tag validation, join-table writes, then starting a Protobuf event contract | [ ] PRs 05 and 06 merged |
| **Day 4** | Batch gRPC RPC design and partial-success semantics, then release-day pairing | Outbox/publisher reimplementation, event tests, then release-day pairing | [ ] PRs 08, 09, 10, and 13 merged; `v0.2.0` tagged |

## Individual Reflection

### Developer A

- [ ] I can explain why this service's write endpoints use an admin key instead of JWTs, and when that choice should change.
- [ ] I can explain how the batch gRPC RPC handles partial failure.
- [ ] I can explain why bulk import and full observability were deferred rather than built shallowly.
- [ ] I reviewed at least one P0 ticket written by Developer B.

**Most important concept learned:**

**Most difficult problem solved:**

**Topic requiring more practice:**

### Developer B

- [ ] I can explain why featured artists are a join table but genre/mood tags are arrays.
- [ ] I can explain the outbox/publisher pattern well enough to have implemented it a second time without re-reading Epic 1's code line by line.
- [ ] I can explain what's left in E2-SS-07/11/12 for Sprint 2, precisely enough that either developer could pick them up cold.
- [ ] I reviewed at least one P0 ticket written by Developer A.

**Most important concept learned:**

**Most difficult problem solved:**

**Topic requiring more practice:**

## Completion Evidence

- [ ] Links to all merged pull requests are recorded.
- [ ] Unit, integration, race, and end-to-end test results are recorded.
- [ ] Migration up/down output is recorded for the `muse_catalog` database.
- [ ] Docker Compose health output is recorded for both services running together.
- [ ] Example HTTP, Kafka, and gRPC requests are documented, even informally.
- [ ] Final CI run is green.
- [ ] Release tag `v0.2.0` points to the verified commit.
- [ ] E2-SS-07, E2-SS-11, and E2-SS-12 are logged as Epic 2 Sprint 2's backlog with their original scope intact.

## Final Outcome

After completing this sprint, both developers should be able to describe
how a second, independently-owned service integrates into the same
platform as the first, and how deliberately scoping a sprint — shipping a
correct core and explicitly deferring the rest — differs from either
overcommitting to the original seven-day plan or silently shipping a
shallower version of everything.
