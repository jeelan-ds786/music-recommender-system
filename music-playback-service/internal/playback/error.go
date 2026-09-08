package playback

import "errors"

// ErrContextTooLarge is returned when the DB rejects context on
// storage-size grounds even though it passed the application-level
// ValidateIngest check — see repository.go's checkViolationContextSize
// constant for why this can happen despite both checks nominally using
// the same 4096-byte limit.
var ErrContextTooLarge = errors.New("context too large")

// ErrSessionNotFound is returned when no playback_events row exists for a
// (session_id, user_id) pair — covers both "session never existed" and
// "session belongs to another user", deliberately indistinguishable to
// the caller (matches this codebase's existing ownership convention:
// non-owned resources return 404, not 403).
var ErrSessionNotFound = errors.New("session not found")

// ErrInvalidCursor is returned for any malformed cursor string — see
// cursor.go's DecodeCursor.
var ErrInvalidCursor = errors.New("invalid cursor")
