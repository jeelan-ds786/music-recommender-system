This documentation captures the setup process for the `music-identity-gatekeeper` service, including the technical hurdles encountered during the containerization process.

---

# Documentation: Setting up `music-identity-gatekeeper`

## 1. Project Initialization

The service was initialized as a Go module to ensure proper dependency management and namespace isolation.

* **Command:** `go mod init github.com/[username]/muse/identity-svc`
* **Structure:**
* `cmd/api/`: Entry point (`main.go`).
* `internal/`: Business logic (auth, db, profile).
* `proto/`: gRPC definitions.
* `migrations/`: SQL migration files.



---

## 2. Containerization Strategy

We utilized a **Multi-Stage Dockerfile** to ensure a small, secure production image.

* **Stage 1 (Builder):** Uses `golang:1.26-alpine` to compile the source code into a static binary.
* **Stage 2 (Runtime):** Uses `gcr.io/distroless/static-debian12`, which excludes shells and package managers to minimize the attack surface.

---

## 3. Issues Faced & Resolutions

| Issue | Root Cause | Resolution |
| --- | --- | --- |
| **Docker Daemon Connection** | Colima (VM) was stuck/not running. | Reset Colima instance: `colima stop`, `colima delete`, `colima start --runtime docker`. |
| **"No such host" (Network)** | Colima VM unable to resolve DNS on corporate network. | Started Colima with explicit DNS: `colima start --dns 8.8.8.8`. |
| **"go.sum" not found** | `go mod tidy` had not been run, creating a mismatch in the build context. | Ran `go mod tidy` and updated `COPY` instruction in Dockerfile to be less strict. |
| **Version Mismatch** | `go.mod` required a newer Go version than the Docker image provided. | Updated `FROM golang:1.22-alpine` to `1.26-alpine`. |
| **"Expected package, found EOF"** | `main.go` file was created but empty. | Added mandatory `package main` and basic logic to `main.go`. |

---

## 4. Build & Run Instructions

### To Build the Service:

```bash
# Navigate to the service directory
cd music-identity-gatekeeper

# Execute build
docker build -t music-identity-gatekeeper .

```

### To Run the Service:

```bash
docker run -p 8080:8080 music-identity-gatekeeper

```

---

## 5. Best Practices Implemented

* **Distroless Images:** Hardened production environment with no shell or package manager.
* **Multi-Stage Build:** Kept the final image size optimized for CI/CD pipelines.
* **DNS Handling:** Documented a stable method for running Docker on restricted corporate networks using Colima.

---

**Do you need any additional documentation on the `internal/auth` logic or how to structure the gRPC proto files next?**




This documentation provides a comprehensive overview of the professional setup for your `music-identity-gatekeeper` service, covering containerization, infrastructure, configuration, and CI/CD pipelines.

---

# Project Setup Documentation: Music Identity Gatekeepergo get github.com/jackc/pgx/v5/stdlib

## 1. Project Architecture

The project follows a modern Go service structure, optimized for scalability and production readiness.

* **Repository Root**: Orchestrates the entire ecosystem (App + Infrastructure).
* **Service Layer (`/music-identity-gatekeeper`)**: Contains the Go source code, build configuration, and service logic.
* **Infrastructure**: Managed via `docker-compose` to ensure local environments mirror production topology.

---

## 2. Containerization Strategy

We implemented a robust, secure, and performant containerization process.

### The Dockerfile

We used a **Multi-Stage Build** to minimize the final image size and improve security:

* **Builder Stage**: Uses `golang:1.26-alpine` to compile the Go source into a static binary.
* **Runtime Stage**: Uses `gcr.io/distroless/static-debian12`, a minimal image that contains no shells or package managers, drastically reducing the attack surface.

### Docker Ignore (`.dockerignore`)

Essential for build performance and security, it prevents sensitive local files (like `.env`, `.git`, or local binaries) from being copied into the container image.

---

## 3. Configuration & Secrets Management

We adopted the **12-Factor App methodology** to ensure the service is portable across environments.

* **Viper**: Used to load configuration values. It prioritizes Environment Variables (best for production/Kubernetes) over local `.env` files (best for development).
* **`.env.example`**: Provided as a template to document required keys (`DB_URL`, `REDIS_URL`, `JWT_SECRET`, `PORT`) without committing sensitive data.
* **`.gitignore`**: Configured to exclude secret files (`.env`), build artifacts, and IDE metadata, ensuring a clean and secure repository.

---

## 4. Infrastructure Orchestration (`docker-compose`)

Local development is automated to run a full stack:

* **Stack**: Includes `Postgres 16` and `Redis 7`.
* **Healthchecks**: Configured to ensure the `identity-svc` only attempts to connect once the database and cache are fully ready, preventing startup race conditions.
* **Environment Injection**: Connection strings are injected dynamically, using Docker service names as hostnames for seamless service discovery.

---

## 5. CI/CD & Code Quality

We implemented a professional-grade CI pipeline using **GitHub Actions**.

* **Automated Validation**: Every `push` or `pull_request` triggers:
* **Golangci-lint**: Enforces high coding standards and catches errors.
* **`go vet` & `go test -race**`: Ensures code correctness and identifies potential concurrency issues in real-time.


* **Pipeline Goal**: To ensure `main` is always in a deployable state, providing a clear "proof of quality" for potential reviewers or interviewers.

---

## 6. Summary of Key Resolutions

During setup, the following technical hurdles were resolved:

| Hurdle | Solution |
| --- | --- |
| **Network/DNS Issues** | Configured Colima with explicit DNS (`8.8.8.8`) to bypass corporate restrictions. |
| **Dependency Versioning** | Upgraded to Go 1.26 to match module requirements. |
| **Empty File Errors** | Initialized `main.go` with proper package structure. |
| **Build Efficiency** | Implemented multi-stage builds and `.dockerignore` for fast, secure builds. |

---

**You now have a production-ready template.** Would you like to move on to implementing the **Database Connection Layer** using `sqlx` or `gorm` to verify the Postgres integration?
## Subscription tier authorization

Access tokens contain the listener's persisted `tier` claim. Premium routes authorize only this signed claim; tier values supplied through headers or request bodies are ignored.

After a subscription upgrade, the profile service returns a newly issued token pair containing the updated tier. Existing access tokens remain valid with their old tier claim until they expire, are refreshed, or are explicitly reissued.

## Google OAuth2

Configure `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, and `GOOGLE_REDIRECT_URL`. Start authentication with `GET /auth/google`; Google redirects to `GET /auth/google/callback`. A successful callback returns the standard access and refresh token pair as JSON. Long-lived tokens are never placed in redirect query parameters.

OAuth state is cryptographically random, stored in Redis only as a SHA-256 hash, expires after 10 minutes, and is consumed atomically on first callback use. Missing, invalid, expired, and replayed states are rejected.

Only Google identities with a verified email are accepted. Existing Google-created accounts may sign in, but an email already attached to a local account returns `409 OAUTH_EMAIL_CONFLICT`; linking requires a separate authenticated flow. Google-created users store no password hash.

## Internal gRPC profile API

The identity service runs `identity.v1.IdentityService` on port `50051` alongside the HTTP server. `GetListenerProfile` accepts a listener UUID and returns the subscription tier, genre seeds, language preferences, followed artist IDs, and liked-song count.

Docker Compose exposes port `50051` only to the internal Compose network; it is not published on the host. Other Compose services can connect to `identity-svc:50051`. Production deployments must authenticate callers with service identity or mTLS and must not expose this API publicly without equivalent access controls.

Generate the checked-in Go client and server code reproducibly with:

```bash
make proto-gen
```

The target requires `protoc` 36.0 and installs pinned versions of `protoc-gen-go` and `protoc-gen-go-grpc`.
