# Sprint 2 - Epic 1 Closeout Learning Outcomes

**Epic:** User Identity & Listener Profile Service
**Duration:** 3 days
**Team:** Developer A and Developer B
**Release target:** `v0.1.0`

## Sprint Learning Goal

By the end of this sprint, the team should be able to close out a service
that was left partially complete after a time-boxed sprint — finishing
what was already designed, adding scope that was missed during planning,
and shipping a first real release — without re-deriving decisions already
made in Sprint 1.

The goal is not only to finish tickets. Each developer should understand
the difference between "planned but not done" (logout, metrics, docs) and
"never planned at all" (playlists), because they require different kinds
of attention: the first needs finishing and rebasing, the second needs a
small design pass before it can be built.

## Languages And Formats

- [ ] **Go:** Extend an existing service with a new domain package (`internal/playlist`) that follows established conventions rather than inventing new ones.
- [ ] **PostgreSQL SQL:** Add two more migrations to an already-running schema; model a cross-service reference column (`song_id` with no local FK) consistently with an existing one (`liked_songs.song_id`).
- [ ] **Protocol Buffers:** Extend an existing event contract with a new message type reusing the same envelope.
- [ ] **Git:** Rebase a long-lived feature branch onto a `main` that has moved significantly since the branch was cut.
- [ ] **Markdown and Mermaid:** Update documentation written in a previous sprint to reflect new endpoints, rather than starting from a blank page.

## Tools And Libraries

| Tool or library | Learning outcome |
|---|---|
| `git rebase` | Land a branch that has drifted behind several merged PRs without losing review history. |
| `pgx/v5` | Add a new table with a deliberately-absent foreign key (a cross-service reference) alongside tables that do use real FKs. |
| `segmentio/kafka-go` | Extend an already-proven outbox/publisher with a new event type instead of building a second publishing path. |
| `prometheus/client_golang` | Instrument a service that was fully built before observability was added to it. |
| `uber-go/zap` | Retrofit structured logging onto existing handlers and services without changing their behavior. |
| Postman and OpenAPI 3.0 | Update, rather than author, API artifacts to reflect new endpoints on a previously-documented service. |

## Core Engineering Outcomes

### Closing Out Partially Complete Work

- [ ] Explain why "implemented on a branch" and "done" are different states, and what has to happen between them.
- [ ] Rebase a feature branch that predates several since-merged PRs, and know when that requires a fresh review versus a fast-forward.
- [ ] Recognize when a ticket needs "finish and merge" effort versus a full new estimate.

### Identifying And Closing A Planning Gap

- [ ] Compare a service's actual scope against its epic-level description to find something that was never planned, not just something left unfinished.
- [ ] Design a new feature (playlists) to fit an existing service's conventions — error shapes, ownership checks, idempotency, event publishing — rather than introducing new patterns for it.
- [ ] Explain why playlist reordering was deliberately left out of this sprint instead of built partially.

### Data Modeling For A New Feature On An Existing Schema

- [ ] Add tables to a schema that's already running in production-like conditions, using additive, reversible migrations.
- [ ] Reuse an established pattern for referencing an entity that lives in a different service's database (`song_id` with no local FK), rather than reinventing it.
- [ ] Enforce ownership at the query/service layer (a playlist not owned by the caller is `404`, not `403`) and explain why that avoids leaking existence.

### Observability On An Already-Built Service

- [ ] Add request IDs, structured logs, and metrics without changing existing endpoint behavior.
- [ ] Verify readiness checks fail correctly when a dependency is down, on a service that was previously running without that check.

### Release Readiness

- [ ] Run a full release checklist for the first time on a service that has never been tagged before, and treat any friction found as a checklist bug to fix, not a step to skip.
- [ ] Extend an end-to-end test to cover new functionality (playlists, logout) without rewriting the parts that already passed.

## Three-Day Learning Tracker

| Day | Developer A learns and demonstrates | Developer B learns and demonstrates | Evidence |
|---|---|---|---|
| **Day 1** | Rebasing a drifted branch, finishing partially-reviewed work | Landing already-built documentation, scoping a new domain package before writing code | [ ] PRs 06 and 12 merged |
| **Day 2** | Retrofitting metrics/logging onto a fully-built service | Playlist schema, CRUD, ownership checks, idempotent song add/remove | [ ] PRs 11 and 14 merged |
| **Day 3** | Running a first-ever release checklist end to end | Extending an existing event publisher with a new event type; updating existing docs for new endpoints | [ ] PRs 15 and 13 merged; `v0.1.0` tagged |

## Individual Reflection

### Developer A

- [ ] I can explain what changed on `main` between when E1-SS-06 was branched and when it merged.
- [ ] I can explain what our readiness check actually verifies and why.
- [ ] I ran the release checklist and know which steps were harder than expected.

**Most important concept learned:**

**Most difficult problem solved:**

**Topic requiring more practice:**

### Developer B

- [ ] I can explain why playlist songs reference the catalog service without a local foreign key.
- [ ] I can explain why an unowned playlist returns 404 instead of 403.
- [ ] I extended the existing event publisher without touching its core publishing logic.

**Most important concept learned:**

**Most difficult problem solved:**

**Topic requiring more practice:**

## Completion Evidence

- [ ] Links to all merged pull requests are recorded.
- [ ] Unit, integration, race, and end-to-end test results are recorded, including the new playlist and logout coverage.
- [ ] Migration up/down output for `000008` and `000009` is recorded.
- [ ] Updated README, OpenAPI, and Postman diffs are recorded.
- [ ] Final CI run is green.
- [ ] Release tag `v0.1.0` points to the verified commit.

## Final Outcome

After completing this sprint, both developers should be able to describe
the full, EPICS.md-complete scope of the User Identity & Listener Profile
Service, explain why it took two sprints instead of one (a time-boxed
super sprint plus a closeout pass), and use that same two-sprint pattern
— ship the core in a super sprint, close the gap afterward — for future
epics.
