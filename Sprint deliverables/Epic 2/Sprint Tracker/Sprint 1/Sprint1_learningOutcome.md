# Epic 2 Sprint 1 - Six-Day Combined Learning Outcomes

**Scope:** Epic 1 remaining work (Phase 1, Days 1-2) + Epic 2 catalog core (Phase 2, Days 3-6)
**Team:** Developer A and Developer B
**Release targets:** `v0.1.0` (Phase 1), `v0.2.0` (Phase 2)

## Sprint Learning Goal

By the end of this sprint, the team should be able to (1) tell the
difference between a ticket that's actually blocked and one that just
needs a mechanical merge step, and schedule accordingly instead of
budgeting dev-days for either by default, and (2) build the core of a
second production service under a compressed timeline right afterward.

The trigger for goal (1): two tickets that looked identical on the
surface ("implemented, not merged") turned out to need completely
different treatment — E1-SS-06 needed real conflict resolution across six
files (verified, then actually done and tested), while E1-SS-12 needed
nothing but a push and a PR. Once that was sorted out, neither consumed
any of this sprint's scheduled capacity — the plan got a day shorter as a
direct result, not because scope was cut.

## Core Engineering Outcomes

### Resolving a Real Merge Conflict (not just diagnosing one)

- [ ] Reconcile two independently-evolved changes to the same file (`internal/auth/middleware.go`, `internal/token/service.go`) so both survive — tier claims and revocation, not one replacing the other.
- [ ] Recognize when one side of a conflict is simply obsolete (the branch's pre-gRPC `http.ListenAndServe` pattern, its stale `REDIS_URL` fallback, its outdated `preference.NewService` call) and discard it rather than trying to merge dead code forward.
- [ ] Verify a conflict resolution with more than a successful `go build` — run the real test suite against a real database, and smoke-test the actual behavior (logout, blacklist TTL, no raw tokens in logs) before calling it resolved.
- [ ] Know where to stop: the resolution lives on its own branch, unmerged into `main` and unpushed, until there's an explicit decision to land it — technical completion and "ready to ship" are different gates.

### Closing Out Epic 1's Real Remaining Work

- [ ] Build playlists (E1-SS-14/15) to fit existing conventions — ownership checks, idempotency, event publishing — as genuinely new scope, distinct from the merge-only tickets.
- [ ] Retrofit metrics/logging onto a fully-built service (E1-SS-11).
- [ ] Run Epic 1's first release checklist, now including logout and playlists in the E2E flow.

### Building The Second Service's Core (Phase 2)

- [ ] Reimplement the outbox/publisher pattern in a second service without repeating E1-SS-09's original bug.
- [ ] Design a batch gRPC RPC with partial-success semantics.
- [ ] Identify what's safe to defer under time pressure versus what can't be cut.

## Six-Day Learning Tracker

| Day | Developer A learns and demonstrates | Developer B learns and demonstrates | Evidence |
|---|---|---|---|
| **1** | Retrofitting metrics/logging onto a fully-built service | Playlist schema, CRUD, ownership checks | [ ] PRs 14 and 11 merged |
| **2** | Landing two merge-only tickets (06, 12) before running Epic 1's first release checklist | Extending the event publisher; updating existing docs | [ ] E1-SS-06 and E1-SS-12 merged to `main`; PRs 15 and 13 merged; `v0.1.0` tagged |
| **3** | Service scaffolding for a second service | SQL relational schema, join tables vs. arrays | [ ] PRs 01 and 02 merged |
| **4** | Album-artist relationship validation | Artist CRUD, pagination | [ ] PRs 03 and 04 merged |
| **5** | Cursor pagination on a second dataset shape | Song CRUD, then a Protobuf event contract | [ ] PRs 05 and 06 merged |
| **6** | Batch gRPC partial-success design, then release pairing | Outbox reimplementation, then release pairing | [ ] PRs 08, 09, 10, 13 merged; `v0.2.0` tagged |

## Individual Reflection

### Developer A

- [ ] I can list the six files that conflicted in E1-SS-06 and explain, for each, why the resolution went the way it did (kept ours, kept theirs, or merged both).
- [ ] I can explain why passing `go build` wasn't enough to call the merge done, and what the smoke test actually proved that the build alone didn't.
- [ ] I know exactly what's left before E1-SS-06 ships: merge into `main`, push to `origin`. Nothing else.

**Most important concept learned:**

**Most difficult problem solved:**

**Topic requiring more practice:**

### Developer B

- [ ] I can explain why E1-SS-12 needed zero additional engineering work, only a process step.
- [ ] I can explain why playlist songs reference the catalog service without a local foreign key.
- [ ] I implemented the outbox pattern a second time without repeating Epic 1's fallback-relay bug.

**Most important concept learned:**

**Most difficult problem solved:**

**Topic requiring more practice:**

## Completion Evidence

- [ ] E1-SS-06 merged into `main` and pushed; E1-SS-12 merged via PR.
- [ ] Links to all remaining merged pull requests are recorded.
- [ ] Unit, integration, race, and end-to-end test results are recorded for both phases.
- [ ] Migration up/down output recorded for both `music-identity-gatekeeper` (through `000009`) and `muse_catalog`.
- [ ] Both CI runs are green; both tags (`v0.1.0`, `v0.2.0`) point to their verified commits.
- [ ] E2-SS-07, E2-SS-11, E2-SS-12 logged as Epic 2 Sprint 2's backlog.

## Final Outcome

After this sprint, both developers should be able to describe not just
what shipped, but how correctly diagnosing two similar-looking tickets
changed the actual schedule — one needed real engineering work with
verification beyond a green build, the other needed none at all — and
why treating them identically at the planning stage would have either
wasted a day or shipped something unverified.
