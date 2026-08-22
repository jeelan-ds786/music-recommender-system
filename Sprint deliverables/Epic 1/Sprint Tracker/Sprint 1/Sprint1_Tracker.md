# Epic 1 - Seven-Day Sprint Tracker

**Sprint goal:** Complete and release the User Identity & Listener Profile Service as `v0.1.0`.

**Team split:**

- **Developer A:** auth, security, OAuth, runtime wiring, gRPC, observability, CI, and release.
- **Developer B:** profile, preferences, likes/follows, Kafka, Docker Compose, and API documentation.

**Working agreement:** B exposes constructors and interfaces. A owns final edits to `cmd/server/main.go`, `go.mod`, and `go.sum`. B owns migrations, event Protobuf definitions, and `docker-compose.yml`.

## Daily Trackers

- [Day 1 - Contracts and auth safety](day-1-contracts-and-auth.md)
- [Day 2 - Profile and tier authorization](day-2-profile-and-tier.md)
- [Day 3 - Preferences and logout](day-3-preferences-and-logout.md)
- [Day 4 - OAuth and Kafka infrastructure](day-4-oauth-and-kafka.md)
- [Day 5 - Events and gRPC](day-5-events-and-grpc.md)
- [Day 6 - Observability and documentation](day-6-observability-and-docs.md)
- [Day 7 - End-to-end release](day-7-release.md)

## Mermaid Timeline

```mermaid
timeline
    title Epic 1 Seven-Day Super Sprint
    Day 1 : A - E1-SS-02 Auth hardening and tests
          : B - E1-SS-01 Preferences schema and contracts
          : Merge 01 then 02
    Day 2 : A - E1-SS-04 Tier claims and middleware
          : B - E1-SS-03 Full profile GET and PATCH
          : Merge 03 then 04
    Day 3 : A - E1-SS-06 Logout and Redis revocation
          : B - E1-SS-05 Onboarding likes and follows
          : Merge 05 then 06
    Day 4 : A - E1-SS-07 Google OAuth2
          : B - E1-SS-08 Kafka contract and infrastructure
          : Merge 07 then 08
    Day 5 : A - E1-SS-10 Internal gRPC API
          : B - E1-SS-09 Registration and preference events
          : Merge 09 then 10
    Day 6 : A - E1-SS-11 Observability and E1-SS-13 setup
          : B - E1-SS-12 Documentation and API artifacts
          : Merge 11 then 12
    Day 7 : A - E1-SS-13 Release captain
          : B - E2E and infrastructure verification
          : Merge 13 then tag v0.1.0
```

## Dependency And Merge Flow

```mermaid
flowchart LR
    T01["01 Schema + contracts"] --> T03["03 Profile GET/PATCH"]
    T02["02 Auth hardening"] --> T06["06 Logout"]
    T02 --> T07["07 OAuth2"]
    T03 --> T04["04 Tier authorization"]
    T03 --> T05["05 Preferences APIs"]
    T05 --> T09["09 Event publishing"]
    T08["08 Kafka infrastructure"] --> T09
    T05 --> T10["10 gRPC API"]
    T09 --> T11["11 Observability"]
    T10 --> T11
    T07 --> T12["12 Docs and API artifacts"]
    T09 --> T12
    T10 --> T12
    T11 --> T13["13 E2E + CI + release"]
    T12 --> T13
```

## Sprint-Level Checklist

- [ ] All 13 tickets merged in the required dependency order.
- [ ] Every P0 ticket reviewed by the developer who did not author it.
- [ ] `go test ./...` passes on the integrated branch every day.
- [ ] `go vet ./...` passes before release.
- [ ] Migration up/down passes against PostgreSQL 16.
- [ ] Docker Compose reports healthy PostgreSQL, Redis, Kafka, HTTP, and gRPC services.
- [ ] End-to-end flow passes twice from a clean environment.
- [ ] README, OpenAPI, Postman, and architecture diagram match the implementation.
- [ ] CI passes on the final default-branch commit.
- [ ] `v0.1.0` is tagged only after the final CI run passes.

## Daily Status Legend

- `[ ]` Not started
- `[~]` In progress; replace manually while tracking
- `[x]` Complete with evidence linked in the daily notes
- `[!]` Blocked; record owner, cause, and next action

At the end of each day, both developers must record the PR link, test command, test result, remaining blocker, and handoff for the next day in that day's tracker.