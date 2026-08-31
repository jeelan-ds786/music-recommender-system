# Epic 2 Sprint 1 - Six-Day Combined Sprint Tracker

**Sprint goal:** Phase 1 (Days 1-2) closes out Epic 1's remaining work and tags `v0.1.0`. Phase 2 (Days 3-6) builds Epic 2's catalog core and tags `v0.2.0`.

**Status of the two carried-over tickets:** E1-SS-06 is actually resolved — merge commit `5a83591` on local branch `fix/e1-ss-06-resolve-conflicts`, verified via `go build`/`go vet`/`go test -race` (incl. real Postgres) plus a live logout smoke test. E1-SS-12 is verified clean on `chore/Doc`. Neither needs scheduled dev time — both are merge-only, tracked as their own row below rather than folded into a day. See `Sprint1_TODOS.md` for full detail.

**Team split:**

- **Developer A:** Phase 1 — E1-SS-11, release-captain E1-SS-13. Phase 2 — service scaffolding, admin auth, albums, browse/filter, gRPC, release-captain E2-SS-13.
- **Developer B:** Phase 1 — E1-SS-14, E1-SS-15, pair on E1-SS-13. Phase 2 — catalog schema, artists, songs, Kafka events.

**Working agreement:** A owns final edits to `cmd/server/main.go`, `go.mod`, and `go.sum`. B owns migrations, event Protobuf definitions, and Docker Compose additions.

## Daily Trackers

- [Day 1 - Metrics and playlist CRUD](day-1-metrics-and-playlists.md)
- [Day 2 - Merge 06/12, playlist events, Epic 1 release (v0.1.0)](day-2-epic1-release.md)
- [Day 3 - Catalog contracts and scaffolding](day-3-catalog-contracts.md)
- [Day 4 - Artist and album APIs](day-4-artist-and-album.md)
- [Day 5 - Song catalog, browse, and Kafka contract](day-5-song-browse-kafka.md)
- [Day 6 - Integration and Epic 2 release (v0.2.0)](day-6-epic2-release.md)

## Mermaid Timeline

```mermaid
timeline
    title Epic 2 Sprint 1 - Six-Day Combined Sprint
    Day 1 : A - E1-SS-11 Metrics and logging
          : B - E1-SS-14 Playlist schema and CRUD
          : Merge 14 then 11
    Day 2 : A - Release captain E1-SS-13 (merges 06 and 12 first)
          : B - Finish E1-SS-15, pair on E1-SS-13
          : Merge 06, 12, 15, then 13, tag v0.1.0
    Day 3 : A - E2-SS-02 Scaffolding and admin auth
          : B - E2-SS-01 Catalog schema
          : Merge 01 then 02
    Day 4 : A - E2-SS-04 Album CRUD
          : B - E2-SS-03 Artist CRUD
          : Merge 03 then 04
    Day 5 : A - E2-SS-06 Browse and filter
          : B - E2-SS-05 Song CRUD, start E2-SS-08
          : Merge 05 then 06
    Day 6 : A - E2-SS-10 gRPC, then release captain
          : B - Finish 08, then E2-SS-09, then pair
          : Merge 08 then 09 then 10 then 13, tag v0.2.0
```

## Dependency And Merge Flow

```mermaid
flowchart LR
    T06["E1-SS-06 Logout (resolved, merge-only)"]
    T12["E1-SS-12 Docs (verified clean, merge-only)"]
    T11["E1-SS-11 Observability"]
    T14["E1-SS-14 Playlist schema + CRUD"]
    T15["E1-SS-15 Playlist events"]
    T13a["E1-SS-13 Epic 1 E2E + v0.1.0"]

    T14 --> T15
    T06 --> T13a
    T11 --> T13a
    T12 --> T13a
    T15 --> T13a

    T13a -.phase boundary.-> T01["E2-SS-01 Schema"]

    T01 --> T03["E2-SS-03 Artist CRUD"]
    T01 --> T08["E2-SS-08 Kafka infra"]
    T03 --> T04["E2-SS-04 Album CRUD"]
    T03 --> T05["E2-SS-05 Song CRUD"]
    T04 --> T05
    T05 --> T06b["E2-SS-06 Browse"]
    T05 --> T10["E2-SS-10 gRPC"]
    T06b --> T10
    T05 --> T09["E2-SS-09 Events"]
    T08 --> T09
    T09 --> T13b["E2-SS-13 Epic 2 E2E + v0.2.0"]
    T10 --> T13b
```

## Sprint-Level Checklist

**Merge-only (no scheduled dev time):**
- [ ] E1-SS-06's `fix/e1-ss-06-resolve-conflicts` branch merged into `main` and pushed to `origin`.
- [ ] E1-SS-12's `chore/Doc` branch pushed to `origin` and merged via reviewed PR.

**Phase 1:**
- [ ] E1-SS-11, E1-SS-14, E1-SS-15 merged.
- [ ] Extended E2E (incl. playlists, logout) passes twice.
- [ ] `v0.1.0` tagged.

**Phase 2:**
- [ ] All 10 in-scope tickets (01-06, 08-10, 13) merged in dependency order.
- [ ] Migration up/down passes for the new `muse_catalog` database.
- [ ] Docker Compose reports healthy Postgres, Kafka, and `catalog-svc`.
- [ ] Basic E2E (create, browse, event, gRPC, update, delete) passes twice.
- [ ] `v0.2.0` tagged.
- [ ] E2-SS-07, E2-SS-11, E2-SS-12 recorded as Epic 2 Sprint 2's backlog.

## Daily Status Legend

- `[ ]` Not started
- `[~]` In progress; replace manually while tracking
- `[x]` Complete with evidence linked in the daily notes
- `[!]` Blocked; record owner, cause, and next action

At the end of each day, both developers must record the PR link, test command, test result, remaining blocker, and handoff for the next day in that day's tracker.
