package playback

import "errors"

// ErrContextTooLarge is returned when the DB rejects context on
// storage-size grounds even though it passed the application-level
// ValidateIngest check — see repository.go's checkViolationContextSize
// constant for why this can happen despite both checks nominally using
// the same 4096-byte limit.
var ErrContextTooLarge = errors.New("context too large")
