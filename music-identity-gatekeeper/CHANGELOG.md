# Changelog of **Music Indentity Gatekeeper**

## v0.0.1-alpha
- feat
    * Add migration `000004_create_preferences` with a new `preferences` table containing:
        - `user_id` as PK/FK to `users(id)` with `ON DELETE CASCADE`
        - `liked_song_ids`, `followed_artist_ids`, `genre_seeds`, and `language_prefs` array columns defaulting to `{}`
        - `updated_at`
        - GIN indexes on `liked_song_ids` and `followed_artist_ids`
        - Reversible via `000004_create_preferences.down.sql`
        - (E1-SS-01)
    * Add `internal/preference` package with:
        - `Preference` domain model
        - Repository methods: `Create`, `GetByUserID`
        - `GetPreferences` service interface
        - Transport-agnostic `PreferenceResponse` DTO
        - `ErrPreferenceNotFound`
        - No HTTP, Kafka, or gRPC dependencies
        - (E1-SS-01)
    * Add `internal/logger` with leveled `Debug`, `Info`, and `Error` logging controlled by `LOG_LEVEL`.
    * Add `internal/reqid` chi middleware to assign or propagate `X-Request-ID` headers and expose the request ID through `reqid.FromContext(ctx)`.
    * Add debug-level lifecycle logging to `auth.Service.Register`, including request ID, DB lookup, result, write completion, and lifecycle completion.

- refactor
    * Update `internal/db.NewPostgresPool` to accept a `pgx.QueryTracer` parameter. Pass `nil` to disable query tracing.
    * Update `auth.NewService` to accept `*logger.Logger` as its third constructor argument.
    * Update `cmd/server/main.go` to wire the logger and query tracer, register `reqid.Middleware`, and log the password-masked Postgres connection target at startup.

- chore
    * Add `internal/db/tracer.go` with a `pgx.QueryTracer` that logs Postgres queries as `READ` or `WRITE` together with bound argument values and query duration.

- test
    * Add unit tests for `internal/preference/service_test.go` using a fake repository without requiring a database.
    * Add integration tests for `internal/preference/repository_test.go` using a real Postgres instance configured through `DB_URL`. Tests are skipped when `DB_URL` is unset.

- fix
    * Fix cleanup ordering in `internal/preference/repository_test.go`. Database cleanup queries were executing after `defer pool.Close()`, causing them to fail silently. Pool cleanup now uses `t.Cleanup()` so cleanup executes in the correct LIFO order.

- build
    * Add `make dev` to run the service locally using `go run ./cmd/server`, equivalent to `npm run dev`.
    * Add `make test` to run the preference test suite with:`go test ./internal/preference/... -v`
    * Add `LOG_LEVEL` to `.env` to control log verbosity: `debug`, `info`, `error`, or `none`. Defaults to `info` when unset.