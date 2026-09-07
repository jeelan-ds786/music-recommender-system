package db

import (
	"testing"
)

func TestOperationOf(t *testing.T) {
	tests := map[string]string{
		"SELECT * FROM playback_events":            "READ",
		" INSERT INTO playback_events VALUES ($1)": "WRITE",
		"UPDATE playback_events SET x = $1":        "WRITE",
		"DELETE FROM playback_events WHERE id=$1":  "WRITE",
		"": "UNKNOWN",
	}

	for sql, want := range tests {
		if got := operationOf(sql); got != want {
			t.Errorf("operationOf(%q) = %q, want %q", sql, got, want)
		}
	}
}
