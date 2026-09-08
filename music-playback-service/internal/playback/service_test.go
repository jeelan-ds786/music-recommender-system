package playback

import (
	"context"
	"errors"
	"testing"

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
