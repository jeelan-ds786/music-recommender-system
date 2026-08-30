# Epic 2 - Four-Day Sprint Tracker

**Sprint goal:** Build and release the core Music Catalog Management Service (schema, CRUD, browse, events, gRPC) as `v0.2.0`. Bulk import, full observability, and full documentation are deferred to Epic 2 Sprint 2.

**Team split:**

- **Developer A:** service scaffolding, admin authorization, albums, browse/filter, gRPC, release.
- **Developer B:** catalog schema, artists, songs, Kafka events, Docker Compose additions.

**Working agreement:** B exposes constructors and interfaces. A owns final edits to `cmd/server/main.go`, `go.mod`, and `go.sum`. B owns migrations, event Protobuf definitions, and the catalog additions to `docker-compose.yml`.

## Daily Trackers

- [Day 1 - Contracts and scaffolding](day-1-contracts-and-scaffolding.md)
- [Day 2 - Artist and album APIs](day-2-artist-and-album.md)
- [Day 3 - Song catalog, browse, and eventing contract](day-3-song-browse-and-kafka-contract.md)
- [Day 4 - Integration and release](day-4-integration-and-release.md)

## Mermaid Timeline

```mermaid
timeline
    title Epic 2 Four-Day Sprint
    Day 1 : A - E2-SS-02 Service scaffolding and admin auth
          : B - E2-SS-01 Catalog schema and domain contracts
          : Merge 01 then 02
    Day 2 : A - E2-SS-04 Album CRUD and artist linking
          : B - E2-SS-03 Artist CRUD and full read API
          : Merge 03 then 04
    Day 3 : A - E2-SS-06 Browse, filter, and pagination API
          : B - E2-SS-05 Song CRUD, then E2-SS-08 Kafka contract
          : Merge 05 then 06
    Day 4 : A - E2-SS-10 gRPC API, then pair as release captain
          : B - Finish E2-SS-08, then E2-SS-09 event publishing, then pair
          : Merge 08 then 09 then 10 then 13, tag v0.2.0
```

## Dependency And Merge Flow

```mermaid
flowchart LR
    T01["01 Schema + contracts"] --> T03["03 Artist CRUD"]
    T01 --> T08["08 Kafka infrastructure"]
    T03 --> T04["04 Album CRUD"]
    T03 --> T05["05 Song CRUD + metadata"]
    T04 --> T05
    T05 --> T06["06 Browse + filter API"]
    T05 --> T10["10 gRPC API"]
    T06 --> T10
    T05 --> T09["09 Event publishing"]
    T08 --> T09
    T09 --> T13["13 Basic E2E + CI + release"]
    T10 --> T13

    T07["07 Bulk import (deferred)"]:::deferred
    T11["11 Full observability (deferred)"]:::deferred
    T12["12 Full docs (deferred)"]:::deferred
    classDef deferred stroke-dasharray: 5 5
```

## Sprint-Level Checklist

- [ ] All 10 in-scope tickets (01-06, 08-10, 13) merged in the required dependency order.
- [ ] Every P0 ticket reviewed by the developer who did not author it.
- [ ] `go test ./...` passes on the integrated branch every day.
- [ ] `go vet ./...` passes before release.
- [ ] Migration up/down passes against PostgreSQL 16 for the new `muse_catalog` database.
- [ ] Docker Compose reports healthy Postgres, Kafka, and `catalog-svc` services.
- [ ] Basic end-to-end flow (create, browse, event, gRPC, update, delete) passes twice from a clean environment.
- [ ] A minimal README exists; full OpenAPI/Postman/diagram are explicitly deferred, not silently missing.
- [ ] CI passes on the final default-branch commit.
- [ ] `v0.2.0` is tagged only after the final CI run passes.
- [ ] E2-SS-07, E2-SS-11, and E2-SS-12 are recorded as the backlog for Epic 2 Sprint 2.

## Daily Status Legend

- `[ ]` Not started
- `[~]` In progress; replace manually while tracking
- `[x]` Complete with evidence linked in the daily notes
- `[!]` Blocked; record owner, cause, and next action

At the end of each day, both developers must record the PR link, test command, test result, remaining blocker, and handoff for the next day in that day's tracker.
