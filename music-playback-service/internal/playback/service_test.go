package playback

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jeelan-ds786/music-recommender-system/music-playback-service/internal/logger"
)

type fakeRepository struct {
	// byKey simulates the (user_id, client_event_id) unique constraint:
	// a second Insert for the same key returns the first call's event ID
	// and inserted=false, exactly like the real ON CONFLICT DO NOTHING +
	// follow-up SELECT.
	byKey map[[2]uuid.UUID]uuid.UUID
	err   error

	// GetSessionEvents fakes — configured directly by tests that need
	// them; zero value (nil, nil, nil, nil) is fine for every test that
	// never calls GetSessionEvents.
	sessionEvents   []Event
	sessionOverview *SessionOverview
	sessionNext     *Cursor
	sessionErr      error
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{byKey: make(map[[2]uuid.UUID]uuid.UUID)}
}

func (r *fakeRepository) Insert(_ context.Context, event *Event) (bool, error) {
	if r.err != nil {
		return false, r.err
	}

	key := [2]uuid.UUID{event.UserID, event.ClientEventID}
	if existingID, ok := r.byKey[key]; ok {
		event.ID = existingID
		return false, nil
	}

	r.byKey[key] = event.ID
	return true, nil
}

func (r *fakeRepository) GetSessionEvents(_ context.Context, _, _ uuid.UUID, _ *Cursor, _ int) ([]Event, *SessionOverview, *Cursor, error) {
	if r.sessionErr != nil {
		return nil, nil, nil, r.sessionErr
	}
	return r.sessionEvents, r.sessionOverview, r.sessionNext, nil
}

func TestIngestSingle(t *testing.T) {
	repo := newFakeRepository()
	svc := NewService(repo, logger.New(logger.LevelNone))
	userID := uuid.New()
	req := validRequest()

	resp, validationErr, err := svc.IngestSingle(context.Background(), userID, req)
	if err != nil || validationErr != nil {
		t.Fatalf("IngestSingle() = %+v, %+v, %v", resp, validationErr, err)
	}
	if resp.Status != IngestStatusAccepted {
		t.Fatalf("status = %s, want %s", resp.Status, IngestStatusAccepted)
	}
	if resp.ClientEventID != req.ClientEventID {
		t.Fatalf("client_event_id = %s, want %s", resp.ClientEventID, req.ClientEventID)
	}

	// Retrying the exact same event is an idempotent success, not an error.
	resp2, validationErr2, err2 := svc.IngestSingle(context.Background(), userID, req)
	if err2 != nil || validationErr2 != nil {
		t.Fatalf("retry IngestSingle() = %+v, %+v, %v", resp2, validationErr2, err2)
	}
	if resp2.Status != IngestStatusDuplicate {
		t.Fatalf("retry status = %s, want %s", resp2.Status, IngestStatusDuplicate)
	}
	if resp2.EventID != resp.EventID {
		t.Fatalf("retry event_id = %s, want the original %s", resp2.EventID, resp.EventID)
	}
}

func TestIngestSingleInvalid(t *testing.T) {
	svc := NewService(newFakeRepository(), logger.New(logger.LevelNone))
	req := validRequest()
	req.EventType = "bogus"

	resp, validationErr, err := svc.IngestSingle(context.Background(), uuid.New(), req)
	if resp != nil || err != nil {
		t.Fatalf("IngestSingle() = %+v, _, %v, want nil response and nil error", resp, err)
	}
	if validationErr == nil || validationErr.Error != "INVALID_EVENT_TYPE" {
		t.Fatalf("validationErr = %+v, want INVALID_EVENT_TYPE", validationErr)
	}
}

// TestIngestSingleContextTooLargeFromDB reproduces the real bug found
// during manual Postman-collection testing: a context payload measured by
// ValidateIngest as exactly at the 4096-byte cap can still be REJECTED by
// Postgres's CHECK constraint, because jsonb re-serializes on storage
// (e.g. adds a space after every ':'), growing a compact payload past the
// DB's limit even though the raw wire bytes passed the Go-side check.
// Before the fix, this surfaced as a raw 500 (repository.Insert's error
// propagated unmapped); it must now come back as the same structured
// CONTEXT_TOO_LARGE validation error a pre-insert rejection would produce.
func TestIngestSingleContextTooLargeFromDB(t *testing.T) {
	repo := newFakeRepository()
	repo.err = ErrContextTooLarge
	svc := NewService(repo, logger.New(logger.LevelNone))

	resp, validationErr, err := svc.IngestSingle(context.Background(), uuid.New(), validRequest())
	if resp != nil || err != nil {
		t.Fatalf("IngestSingle() = %+v, _, %v, want nil response and nil error", resp, err)
	}
	if validationErr == nil || validationErr.Error != "CONTEXT_TOO_LARGE" || validationErr.Field != "context" {
		t.Fatalf("validationErr = %+v, want CONTEXT_TOO_LARGE on field context", validationErr)
	}
}

func TestIngestBatchContextTooLargeFromDBPointsAtIndex(t *testing.T) {
	repo := newFakeRepository()
	userID := uuid.New()

	good := validRequest()
	batch := BatchIngestRequest{Events: []IngestRequest{good, validRequest()}}

	// Only the second event trips the DB-side check — simulate that by
	// making the fake repository fail starting from its second call.
	calls := 0
	failingRepo := &failOnCallRepository{fakeRepository: repo, failOnCall: 2, err: ErrContextTooLarge, calls: &calls}
	svc := NewService(failingRepo, logger.New(logger.LevelNone))

	results, validationErr, err := svc.IngestBatch(context.Background(), userID, batch)
	if results != nil || err != nil {
		t.Fatalf("IngestBatch() = %+v, _, %v, want nil results and nil error", results, err)
	}
	if validationErr == nil || validationErr.Error != "CONTEXT_TOO_LARGE" || validationErr.Field != "events[1].context" {
		t.Fatalf("validationErr = %+v, want CONTEXT_TOO_LARGE on field events[1].context", validationErr)
	}
}

// failOnCallRepository wraps fakeRepository so a specific call index can
// simulate the DB-only failure while earlier calls in the same batch still
// succeed — reproducing the case where prior events in a batch are already
// durably inserted before a later one trips the storage-size edge case.
type failOnCallRepository struct {
	*fakeRepository
	failOnCall int
	err        error
	calls      *int
}

func (r *failOnCallRepository) Insert(ctx context.Context, event *Event) (bool, error) {
	*r.calls++
	if *r.calls == r.failOnCall {
		return false, r.err
	}
	return r.fakeRepository.Insert(ctx, event)
}

func TestIngestSingleRepositoryError(t *testing.T) {
	repo := newFakeRepository()
	repo.err = errors.New("connection reset")
	svc := NewService(repo, logger.New(logger.LevelNone))

	resp, validationErr, err := svc.IngestSingle(context.Background(), uuid.New(), validRequest())
	if resp != nil || validationErr != nil || err == nil {
		t.Fatalf("IngestSingle() = %+v, %+v, %v, want only a repository error", resp, validationErr, err)
	}
}

func TestIngestBatchOrderAndDuplicates(t *testing.T) {
	svc := NewService(newFakeRepository(), logger.New(logger.LevelNone))
	userID := uuid.New()

	first := validRequest()
	second := validRequest()
	batch := BatchIngestRequest{Events: []IngestRequest{first, second}}

	results, validationErr, err := svc.IngestBatch(context.Background(), userID, batch)
	if err != nil || validationErr != nil {
		t.Fatalf("IngestBatch() = %+v, %+v, %v", results, validationErr, err)
	}
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	if results[0].ClientEventID != first.ClientEventID || results[1].ClientEventID != second.ClientEventID {
		t.Fatalf("results out of input order: %+v", results)
	}
	if results[0].Status != IngestStatusAccepted || results[1].Status != IngestStatusAccepted {
		t.Fatalf("expected both accepted on first submission: %+v", results)
	}

	// Resubmit the same batch: both should now report as duplicates, same order.
	results2, validationErr2, err2 := svc.IngestBatch(context.Background(), userID, batch)
	if err2 != nil || validationErr2 != nil {
		t.Fatalf("retry IngestBatch() = %+v, %+v, %v", results2, validationErr2, err2)
	}
	if results2[0].Status != IngestStatusDuplicate || results2[1].Status != IngestStatusDuplicate {
		t.Fatalf("expected both duplicate on resubmission: %+v", results2)
	}
}

func TestIngestBatchAllOrNothingOnMalformed(t *testing.T) {
	repo := newFakeRepository()
	svc := NewService(repo, logger.New(logger.LevelNone))
	userID := uuid.New()

	good := validRequest()
	bad := validRequest()
	bad.PositionMS = -1

	results, validationErr, err := svc.IngestBatch(context.Background(), userID, BatchIngestRequest{
		Events: []IngestRequest{good, bad},
	})
	if results != nil || err != nil {
		t.Fatalf("IngestBatch() = %+v, _, %v, want nil results and nil error", results, err)
	}
	if validationErr == nil || validationErr.Error != "POSITION_MS_NEGATIVE" {
		t.Fatalf("validationErr = %+v, want POSITION_MS_NEGATIVE", validationErr)
	}

	// Nothing should have been written — not even the first, valid event.
	if len(repo.byKey) != 0 {
		t.Fatalf("repo.byKey = %+v, want empty (all-or-nothing on malformed batch)", repo.byKey)
	}
}

func TestGetSessionEventsInvalidCursorRejectedBeforeRepo(t *testing.T) {
	repo := newFakeRepository()
	repo.sessionErr = errors.New("should not be called")
	svc := NewService(repo, logger.New(logger.LevelNone))

	_, err := svc.GetSessionEvents(context.Background(), uuid.New(), uuid.New(), "not-a-valid-cursor", 10)
	if !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("GetSessionEvents() error = %v, want ErrInvalidCursor (decoded before the repo is ever called)", err)
	}
}

func TestGetSessionEventsClampsLimit(t *testing.T) {
	tests := []struct {
		name      string
		input     int
		wantLimit int
	}{
		{name: "zero uses default", input: 0, wantLimit: defaultSessionEventsLimit},
		{name: "negative uses default", input: -5, wantLimit: defaultSessionEventsLimit},
		{name: "over max clamps to max", input: maxSessionEventsLimit + 50, wantLimit: maxSessionEventsLimit},
		{name: "within range passes through", input: 7, wantLimit: 7},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var gotLimit int
			repo := &limitCapturingRepository{
				fakeRepository: newFakeRepository(),
				onCall: func(limit int) {
					gotLimit = limit
				},
			}
			repo.sessionOverview = &SessionOverview{EventCount: 1, OutOfOrder: map[uuid.UUID]bool{}}
			svc := NewService(repo, logger.New(logger.LevelNone))

			if _, err := svc.GetSessionEvents(context.Background(), uuid.New(), uuid.New(), "", test.input); err != nil {
				t.Fatalf("GetSessionEvents() error = %v", err)
			}
			if gotLimit != test.wantLimit {
				t.Fatalf("limit passed to repo = %d, want %d", gotLimit, test.wantLimit)
			}
		})
	}
}

func TestGetSessionEventsPropagatesNotFound(t *testing.T) {
	repo := newFakeRepository()
	repo.sessionErr = ErrSessionNotFound
	svc := NewService(repo, logger.New(logger.LevelNone))

	_, err := svc.GetSessionEvents(context.Background(), uuid.New(), uuid.New(), "", 10)
	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("GetSessionEvents() error = %v, want ErrSessionNotFound", err)
	}
}

func TestGetSessionEventsBuildsNextCursorAndSummary(t *testing.T) {
	userID := uuid.New()
	sessionID := uuid.New()
	event := Event{
		ID: uuid.New(), ClientEventID: uuid.New(), SongID: uuid.New(),
		Type: EventTypePlay, OccurredAt: time.Now(), PositionMS: 0, DeviceType: "mobile",
		IngestedAt: time.Now(),
	}
	next := &Cursor{OccurredAt: event.OccurredAt, ID: event.ID}

	repo := newFakeRepository()
	repo.sessionEvents = []Event{event}
	repo.sessionOverview = &SessionOverview{
		EventCount: 5, StartedAt: event.OccurredAt, EndedAt: event.OccurredAt.Add(time.Minute),
		DurationMS: 60_000, OutOfOrder: map[uuid.UUID]bool{event.ID: true},
	}
	repo.sessionNext = next
	svc := NewService(repo, logger.New(logger.LevelNone))

	page, err := svc.GetSessionEvents(context.Background(), userID, sessionID, "", 1)
	if err != nil {
		t.Fatalf("GetSessionEvents() error = %v", err)
	}
	if page.Session.SessionID != sessionID || page.Session.EventCount != 5 || page.Session.DurationMS != 60_000 {
		t.Fatalf("Session = %+v, want session-wide summary from the overview", page.Session)
	}
	if len(page.Events) != 1 || page.Events[0].EventID != event.ID {
		t.Fatalf("Events = %+v, want the one event from the repo", page.Events)
	}
	if !page.Events[0].Quality.OutOfOrder {
		t.Fatalf("Quality.OutOfOrder = false, want true (event.ID is in overview.OutOfOrder)")
	}
	if page.NextCursor == nil {
		t.Fatal("NextCursor = nil, want non-nil")
	}
	decoded, err := DecodeCursor(*page.NextCursor)
	if err != nil {
		t.Fatalf("DecodeCursor(NextCursor) error = %v", err)
	}
	if decoded.ID != next.ID {
		t.Fatalf("decoded cursor ID = %v, want %v", decoded.ID, next.ID)
	}
}

// limitCapturingRepository wraps fakeRepository to capture the limit
// GetSessionEvents actually receives, for asserting the service's
// clamping behavior independent of the repository's own logic.
type limitCapturingRepository struct {
	*fakeRepository
	onCall func(limit int)
}

func (r *limitCapturingRepository) GetSessionEvents(ctx context.Context, userID, sessionID uuid.UUID, cursor *Cursor, limit int) ([]Event, *SessionOverview, *Cursor, error) {
	r.onCall(limit)
	return r.fakeRepository.GetSessionEvents(ctx, userID, sessionID, cursor, limit)
}

func TestComputeQuality(t *testing.T) {
	now := time.Now()
	durationMS := func(ms int64) *int64 { return &ms }

	tests := []struct {
		name         string
		event        Event
		outOfOrder   bool
		wantLate     bool
		wantPctIsNil bool
		wantPct      float64
	}{
		{
			name:         "on time, no duration -> completion unknown",
			event:        Event{OccurredAt: now, IngestedAt: now.Add(time.Second), PositionMS: 1000, DurationMS: nil},
			wantLate:     false,
			wantPctIsNil: true,
		},
		{
			name:         "zero duration -> completion unknown, not a division-by-zero panic",
			event:        Event{OccurredAt: now, IngestedAt: now.Add(time.Second), PositionMS: 0, DurationMS: durationMS(0)},
			wantLate:     false,
			wantPctIsNil: true,
		},
		{
			name:    "50 percent complete",
			event:   Event{OccurredAt: now, IngestedAt: now.Add(time.Second), PositionMS: 50_000, DurationMS: durationMS(100_000)},
			wantPct: 0.5,
		},
		{
			name:    "position beyond duration clamps to 1.0, not an error",
			event:   Event{OccurredAt: now, IngestedAt: now.Add(time.Second), PositionMS: 150_000, DurationMS: durationMS(100_000)},
			wantPct: 1.0,
		},
		{
			name:     "ingested well within threshold is not late",
			event:    Event{OccurredAt: now, IngestedAt: now.Add(lateThreshold - time.Second)},
			wantLate: false, wantPctIsNil: true,
		},
		{
			name:     "ingested just past threshold is late",
			event:    Event{OccurredAt: now, IngestedAt: now.Add(lateThreshold + time.Second)},
			wantLate: true, wantPctIsNil: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := computeQuality(test.event, test.outOfOrder)
			if got.Late != test.wantLate {
				t.Fatalf("Late = %t, want %t", got.Late, test.wantLate)
			}
			if test.wantPctIsNil {
				if got.CompletionPercentage != nil {
					t.Fatalf("CompletionPercentage = %v, want nil", *got.CompletionPercentage)
				}
				return
			}
			if got.CompletionPercentage == nil {
				t.Fatal("CompletionPercentage = nil, want non-nil")
			}
			if *got.CompletionPercentage != test.wantPct {
				t.Fatalf("CompletionPercentage = %v, want %v", *got.CompletionPercentage, test.wantPct)
			}
		})
	}
}
