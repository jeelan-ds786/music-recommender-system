

## Week 2 — Listener Profile + Preferences + Kafka + gRPC

### Monday — Profile CRUD

**Hours:** 3 hrs **Focus:** schema · api

- [ ] Migration 004 — `preferences` table
    - `user_id FK PK`, `liked_song_ids UUID[]`, `followed_artist_ids UUID[]`, `genre_seeds TEXT[]`, `language_prefs TEXT[]`, `updated_at`
    - Add GIN index on UUID arrays for fast ML feature pulls
- [ ] Wire `GET /me` (auth required)
    - JOIN users + listener_profiles + preferences → return full JSON profile
- [ ] Wire `PATCH /me`
    - Partial update: `display_name`, `avatar_url`, `country`, `language`, `birth_year`
    - Validate country as ISO 3166-1 alpha-2
- [ ] Tests: unauthorized access returns 401 · partial patch doesn't clobber untouched fields

> **Deliverable:** Profile read + update working behind auth middleware

---

### Tuesday — Subscription tier + onboarding preferences

**Hours:** 3 hrs **Focus:** code · api

- [ ] Add subscription tier enum in Postgres
- [ ] Write `SubscriptionService`
    - `Upgrade(userID, tier)` and `GetTier(userID)`
    - Tier injected into JWT claims so downstream services can gate features without a DB call
- [ ] Wire `POST /me/onboarding`
    - Accepts: `genre_seeds` (max 5), `language_prefs`, optional `followed_artist_ids`
    - Called once after registration
    - Used directly by Cold Start service (Epic 11)
- [ ] Add `TierMiddleware`
    - Extracts tier from JWT
    - Blocks premium-only endpoints for free users with structured 403 response
- [ ] Tests: free user blocked on premium endpoint · tier correctly reflected in token after upgrade

> **Deliverable:** Subscription + onboarding flow complete

---

### Wednesday — Liked songs + followed artists

**Hours:** 3 hrs **Focus:** api

- [ ] Wire `POST /me/likes/songs/:id` and `DELETE /me/likes/songs/:id`
    - Append/remove from `liked_song_ids` UUID array atomically
    - Use Postgres array operators: `array_append`, `array_remove`
- [ ] Wire `POST /me/following/artists/:id` and `DELETE /me/following/artists/:id`
    - Same pattern for `followed_artist_ids`
- [ ] Wire `GET /me/likes/songs`
    - Cursor-based pagination (not offset — offset breaks on concurrent inserts)
    - Returns song IDs only — Feature Store hydrates metadata separately
- [ ] Tests
    - Idempotent like: liking twice doesn't duplicate
    - Unlike a non-liked song returns 404
    - Pagination cursor correct

> **Deliverable:** Like/follow endpoints complete and tested

---

### Thursday — Kafka preference events

**Hours:** 3 hrs **Focus:** kafka

- [ ] Add `segmentio/kafka-go` producer
- [ ] Wire Kafka in `docker-compose.yml` (single broker for dev)
- [ ] Topic: `user.preference.updated`
- [ ] Define event schema as Protobuf
    
    ```protobuf
    message PreferenceUpdatedEvent {  string user_id = 1;  repeated string genre_seeds = 2;  repeated string liked_song_ids = 3;  repeated string followed_artist_ids = 4;  google.protobuf.Timestamp updated_at = 5;  string event_type = 6;}
    ```
    
- [ ] Hook Kafka publish into all preference mutations
    - Like/unlike song
    - Follow/unfollow artist
    - Onboarding save
    - Profile language update
    - Publish is **async** — never block HTTP response on Kafka
- [ ] Also publish `user.registered` event on successful registration
    - Feature Store will create initial user feature row on this event
- [ ] Tests: mock Kafka in unit tests · integration test: like a song → verify event lands on topic with correct payload

> **Deliverable:** All preference mutations emit Kafka events

---

### Friday — gRPC internal API + observability

**Hours:** 3 hrs **Focus:** code · infra

- [ ] Define `proto/identity.proto`
    
    ```protobuf
    service IdentityService {  rpc GetListenerProfile(GetProfileRequest) returns (ListenerProfileResponse);}message GetProfileRequest {  string user_id = 1;}message ListenerProfileResponse {  string user_id = 1;  string subscription_tier = 2;  repeated string genre_seeds = 3;  repeated string language_prefs = 4;  repeated string followed_artist_ids = 5;  int32 liked_song_count = 6;}
    ```
    
- [ ] Implement gRPC server on port `50051`
    - Internal network only — no JWT middleware, no auth
    - Called by: Recommendation Engine · Feature Store · Cold Start service
- [ ] Add Prometheus metrics via `prometheus/client_golang`
    - Request count · latency histogram · DB pool wait time · Kafka publish errors
    - Expose `/metrics` endpoint
- [ ] Add structured JSON logging with `uber-go/zap`
    - Log fields: `request_id`, `user_id`, `method`, `latency`, `status`
    - Add request_id middleware (UUID per request)
- [ ] Final sweep
    - [ ] README with local setup guide
    - [ ] Architecture diagram linked
    - [ ] API spec in OpenAPI 3.0
    - [ ] Tag release `v0.1.0`

> **Deliverable:** Epic 1 complete. gRPC + HTTP + Kafka + metrics all live.

---
