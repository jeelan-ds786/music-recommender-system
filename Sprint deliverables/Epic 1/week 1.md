

# Epic 1 — User Identity & Listener Profile Service


## Week 1 — Auth Service + DB + JWT

### Monday — Project scaffold

**Hours:** 3 hrs **Focus:** infra

- [x] Init Go module: `go mod init github.com/yourname/muse/identity-svc`
- [x] Folder structure: `cmd/` · `internal/auth` · `internal/profile` · `internal/db` · `internal/config` · `proto/`
- [x] Write `Dockerfile` (multi-stage: builder → distroless)
- [x] Wire `docker-compose.yml` with Postgres 16 + Redis 7 services
- [x] Add `.env.example` + `config.go` using `github.com/spf13/viper`
    - Keys: `DB_URL`, `REDIS_URL`, `JWT_SECRET`, `PORT`
- [x] Add GitHub Actions CI stub: lint (`golangci-lint`) + `go test ./...` on push

> **Deliverable:** Service boots, connects to Postgres + Redis, CI green

---

### Tuesday — DB schema + migrations

**Hours:** 3 hrs **Focus:** schema

- [x] Install `golang-migrate/migrate`
- [x] Migration 001 — `users` table
    - `id UUID PK`, `email UNIQUE`, `hashed_password`, `auth_provider` (local/google/spotify), `created_at`, `updated_at`
- [x] Migration 002 — `refresh_tokens` table
    - `token_hash VARCHAR PK`, `user_id FK`, `expires_at TIMESTAMPTZ`, `revoked BOOL DEFAULT false`, `created_at`
- [x] Migration 003 — `listener_profiles` table
    - `user_id FK PK`, `display_name`, `avatar_url`, `country CHAR(2)`, `language VARCHAR(10)`, `birth_year SMALLINT`, `subscription_tier ENUM(free/premium/family)`, `updated_at`
- [x] Write `db/db.go` using `pgx/v5` connection pool
- [x] Add `make migrate-up` and `make migrate-down` targets
- [x] Verify migrations apply cleanly and rollback works

> **Deliverable:** All 3 tables live, migrate up/down tested

---

### Wednesday — Register + Login endpoints

**Hours:** 3 hrs **Focus:** api · code

- [x] Add `chi` router
- [x] Wire `POST /auth/register`
    - Validate email format
    - Check for duplicate email
    - bcrypt password (cost 12)
    - Insert user row → return 201
- [x] Wire `POST /auth/login`
    - Lookup by email
    - `bcrypt.CompareHashAndPassword`
    - Return 401 on mismatch with constant-time comparison (no timing attack)
- [x] Add request validation middleware using `go-playground/validator`
    - Return structured error JSON: `{"error": "EMAIL_INVALID", "field": "email"}`
- [ ] Table-driven unit tests
    - Register: duplicate email · weak password · happy path
    - Login: wrong password · unknown email · happy path

> **Deliverable:** Register + login working, tested, bcrypt confirmed in DB

---

### Thursday — JWT issue + refresh token

**Hours:** 3 hrs **Focus:** code · api

- [ ] Implement `TokenService` using `golang-jwt/jwt/v5`
    - Access token: 15 min TTL
    - Claims: `user_id`, `tier`, `iat`, `exp`
    - Sign with HS256 (upgrade to RS256 in Epic 16)
- [ ] Refresh token: 64-byte `crypto/rand` → SHA256 hash stored in Postgres `refresh_tokens`
    - Raw token returned to client once only — never stored raw
- [ ] Wire `POST /auth/refresh`
    - Validate raw token → hash → DB lookup → check expiry + revoked flag
    - Issue new access token + rotate refresh token (old revoked, new issued)
- [ ] Write `AuthMiddleware`
    - Parse Bearer token, validate signature + expiry
    - Inject `user_id` into request context
- [ ] Tests: expired token rejection · tampered signature rejection · refresh rotation

> **Deliverable:** Full auth token lifecycle working with rotation

---

### Friday — OAuth2 + logout + week review

**Hours:** 3 hrs **Focus:** code · infra

- [ ] Wire Google OAuth2 using `golang.org/x/oauth2`
    - Flow: redirect → callback → exchange code → fetch Google profile → upsert user with `auth_provider=google` → issue tokens same as local login
- [ ] Wire `POST /auth/logout`
    - Extract refresh token from body → hash → mark `revoked=true` in DB
    - Redis TTL-based blacklist for access tokens (short-lived, FAANG-grade detail)
- [ ] Integration test: register → login → get access token → call protected endpoint → logout → verify token rejected
- [ ] Week review
    - CI green
    - Docker image builds clean
    - All endpoints in Postman collection committed to repo

> **Deliverable:** Full auth service shippable. Week 1 done.

---

## API surface summary

|Method|Path|Auth|Description|
|---|---|---|---|
|POST|`/auth/register`|None|Register new user|
|POST|`/auth/login`|None|Login, receive tokens|
|POST|`/auth/refresh`|None|Rotate refresh token|
|POST|`/auth/logout`|Bearer|Revoke refresh token|
|GET|`/auth/oauth/google`|None|OAuth2 redirect|
|GET|`/me`|Bearer|Full profile + prefs|
|PATCH|`/me`|Bearer|Update profile fields|
|POST|`/me/onboarding`|Bearer|Set initial preferences|
|POST|`/me/likes/songs/:id`|Bearer|Like a song|
|DELETE|`/me/likes/songs/:id`|Bearer|Unlike a song|
|GET|`/me/likes/songs`|Bearer|List liked songs (cursor paginated)|
|POST|`/me/following/artists/:id`|Bearer|Follow an artist|
|DELETE|`/me/following/artists/:id`|Bearer|Unfollow an artist|
|gRPC|`GetListenerProfile`|Internal|Profile fetch for downstream services|

---

## Kafka events emitted

|Topic|Trigger|
|---|---|
|`user.registered`|Successful registration|
|`user.preference.updated`|Like · unlike · follow · unfollow · onboarding · language update|

---

## Key Go packages

|Package|Use|
|---|---|
|`chi`|HTTP router|
|`pgx/v5`|Postgres driver + pool|
|`golang-migrate/migrate`|DB migrations|
|`golang-jwt/jwt/v5`|JWT issue + validation|
|`golang.org/x/oauth2`|Google OAuth2|
|`go-playground/validator`|Request validation|
|`segmentio/kafka-go`|Kafka producer|
|`google.golang.org/grpc`|gRPC server|
|`spf13/viper`|Config management|
|`uber-go/zap`|Structured logging|
|`prometheus/client_golang`|Metrics|

---

## Daily rhythm

> 30 min — read + plan the day's tasks 2 hrs — code 30 min — write tests + commit

_Don't skip tests. They're resume bullets — "table-driven tests for auth token rotation" is a real interview talking point._


=====================================================================

Here's the full Week 1 plan in Obsidian-ready format:

# 🎵 Muse — Epic 1 · Week 1 Plan
**Service:** User Identity & Listener Profile
**Stack:** Go · PostgreSQL 16 · Redis 7 · Docker · GitHub Actions
**Goal:** Shippable auth service — register, login, JWT, refresh rotation, OAuth2, logout

---

## 🗓️ Day-by-Day Overview

| Day | Focus                    | Deliverable                           |
| --- | ------------------------ | ------------------------------------- |
| Mon | Scaffold + Infra         | Service boots, CI green               |
| Tue | DB Schema + Migrations   | 3 tables live, migrate up/down tested |
| Wed | Register + Login         | Auth endpoints working + tested       |
| Thu | JWT + Refresh Token      | Full token lifecycle with rotation    |
| Fri | OAuth2 + Logout + Review | Shippable auth service                |

---

## 📅 Monday — Project Scaffold
**Hours:** 3h | **Focus:** infra

### Why this day matters
You're setting the structural pattern every future Muse service will copy. Get this right once — folder layout, multi-stage Dockerfile, CI skeleton — and you never have to think about it again. Interviewers also look at repo structure; a clean `cmd/internal/proto` layout signals you know production Go conventions.

### Tasks

- [x] **Init Go module**
  - Run `go mod init github.com/yourname/muse/identity-svc`
  - *Why:* Module path establishes the import convention for the entire service. Using a fake GitHub path is standard even before the repo exists.

- [x] **Create folder structure**
```

cmd/ → main.go entrypoint internal/ auth/ → register, login, token logic profile/ → profile CRUD db/ → pool + query helpers config/ → viper config loader proto/ → gRPC .proto definitions (stub for now) migrations/ → SQL migration files

````
- *Why:* Go's `internal/` package enforces that this code can't be imported by external modules — the right boundary for business logic. `cmd/` isolates the entrypoint from the logic, making the service testable without starting a server.

- [x] **Write multi-stage Dockerfile**
```dockerfile
# Stage 1: builder
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o identity-svc ./cmd

# Stage 2: distroless runtime
FROM gcr.io/distroless/static-debian12
COPY --from=builder /app/identity-svc /
ENTRYPOINT ["/identity-svc"]
````

- _Why:_ Multi-stage keeps the final image tiny (no Go toolchain, no shell). Distroless has no attack surface — no bash, no package manager. This is the FAANG-grade container pattern; a single-stage `FROM golang` image is a red flag in interviews.
    
- [x] **Wire docker-compose.yml**
    
    ```yaml
    services:
      postgres:
        image: postgres:16-alpine
        environment:
          POSTGRES_DB: muse_identity
          POSTGRES_USER: muse
          POSTGRES_PASSWORD: secret
        ports: ["5432:5432"]
      redis:
        image: redis:7-alpine
        ports: ["6379:6379"]
    ```
    
    - _Why:_ Local dev must match prod topology. Running Postgres + Redis in Compose from day 1 means your tests hit real infrastructure, not mocks — much stronger interview story.
- [x] **Add .env.example + config.go (Viper)**
    
    - Keys: `DB_URL`, `REDIS_URL`, `JWT_SECRET`, `PORT`
    - Load via `viper.AutomaticEnv()` + `.env` file fallback
    - _Why:_ Viper handles 12-factor config (env vars > config files > defaults). Hard-coding secrets is an instant reject in any interview code review.
- [x] **GitHub Actions CI stub**
    
    ```yaml
    on: [push, pull_request]
    jobs:
      ci:
        runs-on: ubuntu-latest
        steps:
          - uses: actions/checkout@v4
          - uses: actions/setup-go@v5
            with: { go-version: '1.22' }
          - run: go vet ./...
          - run: go test ./...
    ```
    
    - Add `golangci-lint` action after core tests pass
    - _Why:_ CI from day 1 is a professional signal. Every commit being validated means you can honestly say "all tests pass on main" in an interview — and you have proof.

> ✅ **Deliverable:** `go run ./cmd` connects to Postgres + Redis. CI pipeline green on push.

---

## 📅 Tuesday — DB Schema + Migrations

**Hours:** 3h | **Focus:** schema

### Why this day matters

The schema you design today will be queried by the ML recommendation pipeline in Epic 7, the Kafka feature store, and every downstream service. Getting the types right (UUIDs not ints, TIMESTAMPTZ not TIMESTAMP, proper FK constraints) is what separates a senior engineer's DB design from a tutorial's. `golang-migrate` with numbered SQL files is the industry standard — no ORM magic that hides what SQL is actually running.

### Tasks

- [x] **Install golang-migrate**
    
    - `go get github.com/golang-migrate/migrate/v4`
    - Add CLI: `brew install golang-migrate` (or binary)
    - _Why:_ Version-controlled migrations mean every environment (local, staging, prod) runs the exact same schema evolution. You can demo rollback in an interview — that's a concrete point that almost no junior candidate can show.
- [x] **Migration 001 — users table**
    
    ```sql
    CREATE TABLE users (
      id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
      email           VARCHAR(255) NOT NULL UNIQUE,
      hashed_password VARCHAR(255),           -- NULL for OAuth-only users
      auth_provider   VARCHAR(20) NOT NULL DEFAULT 'local', -- local | google | spotify
      created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
      updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
    );
    CREATE INDEX idx_users_email ON users(email);
    ```
    
    - _Why:_ `gen_random_uuid()` is native in Postgres 13+ — no extension needed. `hashed_password` is nullable because Google/Spotify OAuth users have no password. `TIMESTAMPTZ` stores timezone-aware timestamps — always use this over `TIMESTAMP` in distributed systems.
- [x] **Migration 002 — refresh_tokens table**
    
    ```sql
    CREATE TABLE refresh_tokens (
      token_hash  VARCHAR(64) PRIMARY KEY,  -- SHA256 hex of raw token
      user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
      expires_at  TIMESTAMPTZ NOT NULL,
      revoked     BOOLEAN NOT NULL DEFAULT false,
      created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
    );
    CREATE INDEX idx_refresh_tokens_user ON refresh_tokens(user_id);
    ```
    
    - _Why:_ Storing the hash, not the raw token, means a DB breach doesn't expose valid refresh tokens. `ON DELETE CASCADE` cleans up tokens when a user is deleted — no orphan rows. This is the detail that shows you've thought about security at the data layer.
- [x] **Migration 003 — listener_profiles table**
    
    ```sql
    CREATE TYPE subscription_tier AS ENUM ('free', 'premium', 'family');
    
    CREATE TABLE listener_profiles (
      user_id           UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
      display_name      VARCHAR(100),
      avatar_url        TEXT,
      country           CHAR(2),          -- ISO 3166-1 alpha-2
      language          VARCHAR(10),      -- BCP 47 e.g. "en-IN"
      birth_year        SMALLINT,
      subscription_tier subscription_tier NOT NULL DEFAULT 'free',
      updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
    );
    ```
    
    - _Why:_ `ENUM` for subscription tier means the DB itself enforces valid values — no application-layer guard needed. `CHAR(2)` for country codes is a type contract (always exactly 2 chars). `birth_year` as `SMALLINT` instead of full DOB avoids storing PII you don't need.
- [x] **Write db/db.go with pgx/v5 pool**
    
    ```go
    func NewPool(ctx context.Context, dbURL string) (*pgxpool.Pool, error) {
      config, err := pgxpool.ParseConfig(dbURL)
      // set MaxConns, MinConns, MaxConnLifetime
      return pgxpool.NewWithConfig(ctx, config)
    }
    ```
    
    - _Why:_ `pgx/v5` is the fastest Postgres driver for Go. Using a pool (not a single connection) handles concurrent requests without opening a new connection per request — mandatory for any service handling real load.
- [ ] **Add Makefile targets**
    
    ```makefile
    migrate-up:
      migrate -path ./migrations -database $(DB_URL) up
    migrate-down:
      migrate -path ./migrations -database $(DB_URL) down 1
    ```
    
    - _Why:_ `down 1` (one step) vs `down` (nuke everything) is the safe default — prevents accidental full rollback in prod.
- [x] **Verify migrations**
    
    - Run `make migrate-up` → inspect tables in psql
    - Run `make migrate-down` × 3 → verify clean rollback
    - _Why:_ If your down migrations are broken you'll find out at the worst time. Verify now.

> ✅ **Deliverable:** All 3 tables in Postgres. `migrate up` and `migrate down` tested and confirmed working.

---

## 📅 Wednesday — Register + Login Endpoints

**Hours:** 3h | **Focus:** api · code

### Why this day matters

Register and login are the two most-scrutinised endpoints in any auth system. Every security decision here — bcrypt cost, timing-safe comparison, structured error responses, input validation before DB hit — will come up in interviews. This is also where you establish the request/response conventions every future endpoint will follow.

### Tasks

- [x] **Add chi router**
    
    - `go get github.com/go-chi/chi/v5`
    - Wire in `cmd/main.go`: `r := chi.NewRouter()`
    - _Why:_ `chi` is idiomatic, stdlib-compatible (`http.Handler`), and has the best middleware composability in the Go ecosystem. No magic — you can explain every line. `gorilla/mux` is older and heavier; `gin` is fine but not what Go engineers at top companies typically reach for.
- [x] **POST /auth/register**
    
    ```
    Validate input → check duplicate email → bcrypt → insert → 201
    ```
    
    - Validate email format with `go-playground/validator` tag `email`
    - Check for duplicate before hashing (save CPU on invalid requests)
    - `bcrypt.GenerateFromPassword([]byte(password), 12)`
    - Return `201 Created` with `user_id` and `created_at`
    - _Why:_ bcrypt cost 12 is the current recommended floor — cost 10 is too fast on modern hardware, cost 14 adds ~600ms latency. The duplicate check before hashing is a micro-optimisation that avoids the 200ms bcrypt computation on emails you're going to reject anyway.
- [x] **POST /auth/login**
    
    ```
    Lookup by email → compare hash → 401 on any mismatch → issue tokens
    ```
    
    - Always call `bcrypt.CompareHashAndPassword` even when email not found (dummy hash compare)
    - Return identical 401 for "wrong password" and "unknown email"
    - _Why:_ Returning different errors for wrong-password vs unknown-email is a user enumeration vulnerability — attackers can probe which emails are registered. The dummy compare on missing email ensures constant-time response, defeating timing attacks.
- [ ] **Request validation middleware**
    
    ```go
    type RegisterRequest struct {
      Email    string `json:"email" validate:"required,email"`
      Password string `json:"password" validate:"required,min=8"`
    }
    ```
    
    - Return structured errors:
        
        ```json
        {"error": "VALIDATION_FAILED", "fields": [{"field": "email", "message": "EMAIL_INVALID"}]}
        ```
        
    - _Why:_ Structured error codes (not free-text messages) let frontend clients handle errors programmatically. `VALIDATION_FAILED` + field-level details is the pattern used by Stripe, Twilio, every serious API. Free-text like `"invalid email address"` is a junior pattern.
- [x] **Table-driven unit tests**
    
    ```go
    tests := []struct {
      name     string
      input    RegisterRequest
      wantCode int
    }{
      {"happy path", {Email: "a@b.com", Password: "securepass"}, 201},
      {"duplicate email", {Email: "existing@b.com", Password: "securepass"}, 409},
      {"invalid email", {Email: "notanemail", Password: "securepass"}, 400},
      {"short password", {Email: "a@b.com", Password: "abc"}, 400},
    }
    ```
    
    - Mirror the same pattern for login tests
    - _Why:_ Table-driven tests are idiomatic Go (the stdlib itself uses this pattern). They're compact, easy to extend, and signal Go fluency. "I wrote table-driven tests for the auth layer" is a concrete interview bullet.

> ✅ **Deliverable:** Register + login endpoints working. bcrypt hash visible in DB. Tests passing.

---

## 📅 Thursday — JWT Issue + Refresh Token

**Hours:** 3h | **Focus:** code · api

### Why this day matters

The tutorial pattern is one long-lived JWT. The production pattern — short-lived access token + rotating refresh token with revocation — is what this day implements. Refresh token rotation (old token revoked on use, new one issued) defeats token theft. This is the single highest-signal auth detail you can demonstrate in an interview; most candidates don't implement it.

### Tasks

- [ ] **Implement TokenService**
    
    ```go
    type TokenService struct {
      secret []byte
      db     *pgxpool.Pool
    }
    
    func (s *TokenService) IssueAccessToken(userID uuid.UUID, tier string) (string, error) {
      claims := jwt.MapClaims{
        "user_id": userID.String(),
        "tier":    tier,
        "iat":     time.Now().Unix(),
        "exp":     time.Now().Add(15 * time.Minute).Unix(),
      }
      return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
    }
    ```
    
    - _Why:_ 15-minute TTL means a stolen access token is useless within 15 minutes. Including `tier` in claims lets downstream services gate features without a DB round-trip. HS256 now, RS256 later (Epic 16) when you need public key verification across services.
- [ ] **Refresh token — generation + storage**
    
    ```go
    raw := make([]byte, 64)
    crypto/rand.Read(raw)
    rawStr := base64.URLEncoding.EncodeToString(raw)
    hash := sha256.Sum256([]byte(rawStr))
    hashHex := hex.EncodeToString(hash[:])
    // Store hashHex in DB, return rawStr to client once
    ```
    
    - _Why:_ `crypto/rand` is cryptographically secure (not `math/rand`). SHA256 of the raw token is stored — if the DB is breached, the attacker has hashes, not valid tokens. The raw token is returned to the client exactly once, never persisted. This is the same pattern used by GitHub's personal access tokens.
- [ ] **POST /auth/refresh — token rotation**
    
    ```
    Receive raw token → SHA256 hash → DB lookup
    → check revoked flag → check expires_at
    → mark old token revoked
    → issue new refresh token + new access token
    → return both
    ```
    
    - Do the revocation + new token insert in a single DB transaction
    - _Why:_ If you revoke the old token but crash before inserting the new one, the user is permanently logged out. Transaction makes this atomic. This edge case is an interview trap question — having thought about it shows senior-level thinking.
- [ ] **AuthMiddleware**
    
    ```go
    func AuthMiddleware(secret []byte) func(http.Handler) http.Handler {
      return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
          token := extractBearer(r.Header.Get("Authorization"))
          claims, err := validateJWT(token, secret)
          if err != nil { http.Error(w, "UNAUTHORIZED", 401); return }
          ctx := context.WithValue(r.Context(), userIDKey, claims["user_id"])
          next.ServeHTTP(w, r.WithContext(ctx))
        })
      }
    }
    ```
    
    - _Why:_ Middleware keeps auth logic out of every handler. `context.WithValue` is the idiomatic Go way to pass request-scoped data downstream. Handler just calls `r.Context().Value(userIDKey)` — no auth logic leaks in.
- [ ] **Tests**
    
    - Expired access token → 401
    - Tampered signature → 401
    - Valid refresh → new tokens issued, old token revoked in DB
    - Reuse of revoked refresh token → 401
    - _Why:_ The revoked-token-reuse test is the critical one. If your system allows a previously used refresh token to work again, it's broken. Test this explicitly.

> ✅ **Deliverable:** Full token lifecycle — issue, validate, rotate, revoke — working with tests.

---

## 📅 Friday — OAuth2 + Logout + Week Review

**Hours:** 3h | **Focus:** code · infra · review

### Why this day matters

OAuth2 is non-negotiable for any consumer product. The Google flow here follows the standard redirect → callback → token exchange pattern. Logout with both DB revocation (refresh token) and Redis blacklist (access token) is FAANG-grade — most systems only do one. The week review locks in CI hygiene and gives you a clean commit history to point at in interviews.

### Tasks

- [ ] **Google OAuth2 flow**
    
    ```
    GET /auth/oauth/google
    → redirect to Google consent screen
    
    GET /auth/oauth/google/callback?code=...
    → exchange code for Google token
    → fetch Google profile (email, name, avatar)
    → upsert user with auth_provider='google'
    → issue same access + refresh tokens as local login
    → redirect to frontend with tokens
    ```
    
    - Use `golang.org/x/oauth2/google`
    - Upsert: `INSERT ... ON CONFLICT (email) DO UPDATE SET updated_at = NOW()`
    - _Why:_ The upsert pattern handles the case where a user first registered locally then tries Google login with the same email — you merge the accounts instead of creating a duplicate. This is a real product edge case that interviewers respect when you mention it.
- [ ] **POST /auth/logout**
    
    ```go
    // 1. Revoke refresh token in DB
    UPDATE refresh_tokens SET revoked = true WHERE token_hash = $1
    
    // 2. Blacklist access token in Redis
    redis.SetEx(ctx, "blacklist:"+accessToken, "1", timeUntilExpiry)
    ```
    
    - Update `AuthMiddleware` to check Redis blacklist before accepting token
    - _Why:_ Access tokens are stateless JWTs — you can't "delete" them. But you can blacklist the JTI (or full token) in Redis with a TTL matching the token's remaining lifetime. After the TTL, the token would have expired anyway, so the Redis key cleans itself up. This is the only way to implement true logout with short-lived JWTs.
- [ ] **Integration test — full auth flow**
    
    ```
    Register → Login → get access token
    → call GET /me (protected) → 200
    → Logout
    → call GET /me with same access token → 401
    → attempt refresh with revoked refresh token → 401
    ```
    
    - _Why:_ This is your demo script. When an interviewer asks "how did you test this?" you walk through this exact sequence. It covers every state the auth system can be in.
- [ ] **Week review checklist**
    
    - [ ] All tests passing locally + in CI
    - [ ] Docker image builds: `docker build -t muse-identity .`
    - [ ] Compose up: `docker-compose up` boots Postgres + Redis + service cleanly
    - [ ] Postman collection exported and committed to `docs/postman/`
    - [ ] README updated: how to run locally, env vars needed, migrate command
    - _Why:_ The README is what an interviewer clones and runs. If it doesn't work in 5 minutes they stop looking. Write it now while the setup is fresh.

> ✅ **Deliverable:** Full shippable auth service. CI green. Docker clean. Week 1 done.

---

## 📦 Key Packages — Week 1

|Package|Use|Why not the alternative|
|---|---|---|
|`chi`|HTTP router|Lighter than `gin`, stdlib-compatible, composable middleware|
|`pgx/v5`|Postgres driver|Fastest Go PG driver; `database/sql` is slower and less ergonomic|
|`golang-migrate/migrate`|DB migrations|Plain SQL files — no ORM magic hiding what's running|
|`golang-jwt/jwt/v5`|JWT|Maintained v5 with proper `Claims` interface|
|`golang.org/x/oauth2`|OAuth2|Official Google lib — no third-party trust needed|
|`go-playground/validator`|Request validation|Struct tags — validation lives with the type definition|
|`spf13/viper`|Config|12-factor: env vars override config file automatically|
|`bcrypt`|Password hashing|Stdlib — no extra dep; adaptive cost factor built in|
|`crypto/rand`|Secure random|Not `math/rand` — cryptographically secure, non-negotiable for tokens|

---

## ⏱️ Daily Rhythm

```
30 min  — read today's plan, understand the why, open relevant docs
2 hrs   — code (follow tasks top to bottom)
30 min  — write tests + commit with a descriptive message
```

> Commit message format: `feat(auth): implement refresh token rotation with revocation` This is conventional commits — standard in any team codebase.

---

## 🎯 Interview Talking Points Unlocked This Week

- Multi-stage distroless Docker build
- golang-migrate versioned SQL migrations with rollback
- bcrypt cost 12 + dummy compare to prevent user enumeration + timing attacks
- Structured error responses with machine-readable codes
- Table-driven unit tests (idiomatic Go)
- SHA256-hashed refresh tokens — raw token never persisted
- Refresh token rotation in a single DB transaction (atomic revocation + reissue)
- Redis TTL blacklist for access token revocation on logout
- Google OAuth upsert — account merging on email collision

---

_Week 2 →_ Profile CRUD · Subscription tier · Liked songs (cursor-paginated) · Kafka events · gRPC internal endpoint

```

Paste that directly into Obsidian — all the code blocks, tables, and checkboxes will render clean. The "Why" callouts under every task are the interview ammunition. Lmk when u want Week 2 in the same format.
```