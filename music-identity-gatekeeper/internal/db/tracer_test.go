package db

import (
	"strings"
	"testing"
)

func TestFormatArgs_NoArgs(t *testing.T) {
	if got := formatArgs(nil); got != "-" {
		t.Errorf("formatArgs(nil) = %q, want %q", got, "-")
	}
}

func TestFormatArgs_ByteSliceRendersAsReadableString(t *testing.T) {
	payload := []byte(`{"event_id":"abc"}`)

	got := formatArgs([]any{"user.registered", payload})

	want := `$1=user.registered, $2={"event_id":"abc"}`
	if got != want {
		t.Errorf("formatArgs = %q, want %q", got, want)
	}
	if strings.Contains(got, "[") {
		t.Errorf("formatArgs still looks like a decimal byte dump: %q", got)
	}
}

func TestFormatArgs_NonByteArgsUnaffected(t *testing.T) {
	got := formatArgs([]any{42, "hello", true})

	want := "$1=42, $2=hello, $3=true"
	if got != want {
		t.Errorf("formatArgs = %q, want %q", got, want)
	}
}

func TestFormatArgs_TruncatesLongValues(t *testing.T) {
	long := strings.Repeat("x", maxArgLen+50)

	got := formatArg(long)

	if !strings.HasSuffix(got, "...(truncated)") {
		t.Errorf("expected truncation suffix, got suffix %q", got[len(got)-20:])
	}
	if len(got) != maxArgLen+len("...(truncated)") {
		t.Errorf("unexpected truncated length: %d", len(got))
	}
}
