package db

import (
	"testing"
)

func TestOperationOf(t *testing.T) {
	tests := map[string]string{
		"SELECT * FROM users":            "READ",
		" INSERT INTO users VALUES ($1)": "WRITE",
		"UPDATE users SET email = $1":    "WRITE",
		"DELETE FROM users WHERE id=$1":  "WRITE",
		"":                               "UNKNOWN",
	}

	for sql, want := range tests {
		if got := operationOf(sql); got != want {
			t.Errorf("operationOf(%q) = %q, want %q", sql, got, want)
		}
	}
}
