package playback

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

func validRequest() IngestRequest {
	return IngestRequest{
		ClientEventID: uuid.New(),
		SongID:        uuid.New(),
		SessionID:     uuid.New(),
		EventType:     EventTypePlay,
		OccurredAt:    time.Now(),
		PositionMS:    1000,
		DeviceType:    "mobile",
	}
}

func TestValidateIngest(t *testing.T) {
	now := time.Now()
	negativeDuration := int64(-1)

	tests := []struct {
		name    string
		mutate  func(*IngestRequest)
		wantErr string
	}{
		{name: "valid", mutate: func(*IngestRequest) {}, wantErr: ""},
		{name: "missing client event id", mutate: func(r *IngestRequest) { r.ClientEventID = uuid.Nil }, wantErr: "CLIENT_EVENT_ID_REQUIRED"},
		{name: "missing song id", mutate: func(r *IngestRequest) { r.SongID = uuid.Nil }, wantErr: "SONG_ID_REQUIRED"},
		{name: "missing session id", mutate: func(r *IngestRequest) { r.SessionID = uuid.Nil }, wantErr: "SESSION_ID_REQUIRED"},
		{name: "invalid event type", mutate: func(r *IngestRequest) { r.EventType = "bogus" }, wantErr: "INVALID_EVENT_TYPE"},
		{name: "zero occurred_at", mutate: func(r *IngestRequest) { r.OccurredAt = time.Time{} }, wantErr: "OCCURRED_AT_REQUIRED"},
		{name: "occurred_at too far in future", mutate: func(r *IngestRequest) { r.OccurredAt = now.Add(25 * time.Hour) }, wantErr: "OCCURRED_AT_TOO_FAR_IN_FUTURE"},
		{name: "occurred_at 24h future is fine", mutate: func(r *IngestRequest) { r.OccurredAt = now.Add(23 * time.Hour) }, wantErr: ""},
		{name: "occurred_at in the past is fine", mutate: func(r *IngestRequest) { r.OccurredAt = now.Add(-30 * 24 * time.Hour) }, wantErr: ""},
		{name: "negative position", mutate: func(r *IngestRequest) { r.PositionMS = -1 }, wantErr: "POSITION_MS_NEGATIVE"},
		{name: "negative duration", mutate: func(r *IngestRequest) { r.DurationMS = &negativeDuration }, wantErr: "DURATION_MS_NEGATIVE"},
		{name: "blank device type", mutate: func(r *IngestRequest) { r.DeviceType = "   " }, wantErr: "DEVICE_TYPE_REQUIRED"},
		{name: "context too large", mutate: func(r *IngestRequest) { r.Context = json.RawMessage(make([]byte, MaxContextBytes+1)) }, wantErr: "CONTEXT_TOO_LARGE"},
		{name: "context at exactly the cap is fine", mutate: func(r *IngestRequest) { r.Context = json.RawMessage(make([]byte, MaxContextBytes)) }, wantErr: ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := validRequest()
			test.mutate(&req)

			got := ValidateIngest(req, now, "")

			if test.wantErr == "" {
				if got != nil {
					t.Fatalf("ValidateIngest() = %+v, want nil", got)
				}
				return
			}
			if got == nil || got.Error != test.wantErr {
				t.Fatalf("ValidateIngest() = %+v, want error %q", got, test.wantErr)
			}
		})
	}
}

func TestValidateIngestFieldPrefix(t *testing.T) {
	req := validRequest()
	req.EventType = "bogus"

	got := ValidateIngest(req, time.Now(), "events[3].")
	if got == nil || got.Field != "events[3].event_type" {
		t.Fatalf("ValidateIngest() field = %+v, want events[3].event_type", got)
	}
}

func TestValidateBatch(t *testing.T) {
	now := time.Now()

	t.Run("empty batch rejected", func(t *testing.T) {
		got := ValidateBatch(BatchIngestRequest{}, now)
		if got == nil || got.Error != "BATCH_EMPTY" {
			t.Fatalf("ValidateBatch() = %+v, want BATCH_EMPTY", got)
		}
	})

	t.Run("batch over cap rejected", func(t *testing.T) {
		events := make([]IngestRequest, MaxBatchSize+1)
		for i := range events {
			events[i] = validRequest()
		}
		got := ValidateBatch(BatchIngestRequest{Events: events}, now)
		if got == nil || got.Error != "BATCH_TOO_LARGE" {
			t.Fatalf("ValidateBatch() = %+v, want BATCH_TOO_LARGE", got)
		}
	})

	t.Run("batch at exactly the cap is fine", func(t *testing.T) {
		events := make([]IngestRequest, MaxBatchSize)
		for i := range events {
			events[i] = validRequest()
		}
		if got := ValidateBatch(BatchIngestRequest{Events: events}, now); got != nil {
			t.Fatalf("ValidateBatch() = %+v, want nil", got)
		}
	})

	t.Run("one malformed event aborts the whole batch, points at its index", func(t *testing.T) {
		events := []IngestRequest{validRequest(), validRequest(), validRequest()}
		events[2].PositionMS = -1

		got := ValidateBatch(BatchIngestRequest{Events: events}, now)
		if got == nil || got.Error != "POSITION_MS_NEGATIVE" || got.Field != "events[2].position_ms" {
			t.Fatalf("ValidateBatch() = %+v, want POSITION_MS_NEGATIVE at events[2].position_ms", got)
		}
	})
}
