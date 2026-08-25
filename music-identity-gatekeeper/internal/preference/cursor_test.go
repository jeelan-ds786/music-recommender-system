package preference

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCursor_RoundTrip(t *testing.T) {
	want := Cursor{
		CreatedAt: time.Now().UTC(),
		ID:        uuid.New(),
	}

	encoded := EncodeCursor(want)

	got, err := DecodeCursor(encoded)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !got.CreatedAt.Equal(want.CreatedAt) {
		t.Errorf("CreatedAt mismatch: got %v want %v", got.CreatedAt, want.CreatedAt)
	}
	if got.ID != want.ID {
		t.Errorf("ID mismatch: got %v want %v", got.ID, want.ID)
	}
}

func TestCursor_DecodeMalformed(t *testing.T) {
	cases := []string{
		"",
		"not-base64!!!",
		"bm8tY29sb24taGVyZQ==", // valid base64, no ":" separator
	}

	for _, c := range cases {
		if _, err := DecodeCursor(c); !errors.Is(err, ErrInvalidCursor) {
			t.Errorf("DecodeCursor(%q): expected ErrInvalidCursor, got %v", c, err)
		}
	}
}
