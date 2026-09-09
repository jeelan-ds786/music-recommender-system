package playback

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCursorRoundTrip(t *testing.T) {
	c := Cursor{OccurredAt: time.Now().UTC(), ID: uuid.New()}

	decoded, err := DecodeCursor(EncodeCursor(c))
	if err != nil {
		t.Fatalf("DecodeCursor() error = %v", err)
	}
	if !decoded.OccurredAt.Equal(c.OccurredAt) {
		t.Fatalf("OccurredAt = %v, want %v", decoded.OccurredAt, c.OccurredAt)
	}
	if decoded.ID != c.ID {
		t.Fatalf("ID = %v, want %v", decoded.ID, c.ID)
	}
}

func TestCursorDecodeMalformed(t *testing.T) {
	tests := []string{
		"",
		"not-base64!!!",
		"bm8tY29sb24taGVyZQ", // valid base64, decodes to "no-colon-here"
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			_, err := DecodeCursor(input)
			if !errors.Is(err, ErrInvalidCursor) {
				t.Fatalf("DecodeCursor(%q) error = %v, want ErrInvalidCursor", input, err)
			}
		})
	}
}
