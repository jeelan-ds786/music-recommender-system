package preference

import (
	"encoding/base64"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Cursor identifies a position in the (created_at DESC, id DESC) ordering
// used by ListLikedSongs. It's opaque to callers: encode it to a string for
// the API response, decode a caller-supplied string back before querying.
type Cursor struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

func EncodeCursor(c Cursor) string {
	raw := strconv.FormatInt(c.CreatedAt.UnixNano(), 10) + ":" + c.ID.String()
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func DecodeCursor(s string) (*Cursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, ErrInvalidCursor
	}

	parts := strings.SplitN(string(raw), ":", 2)
	if len(parts) != 2 {
		return nil, ErrInvalidCursor
	}

	nanos, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return nil, ErrInvalidCursor
	}

	id, err := uuid.Parse(parts[1])
	if err != nil {
		return nil, ErrInvalidCursor
	}

	return &Cursor{
		CreatedAt: time.Unix(0, nanos).UTC(),
		ID:        id,
	}, nil
}
