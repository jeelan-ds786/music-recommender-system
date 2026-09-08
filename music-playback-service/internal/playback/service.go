package playback

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/jeelan-ds786/music-recommender-system/music-playback-service/internal/logger"
	"github.com/jeelan-ds786/music-recommender-system/music-playback-service/internal/reqid"
)

type Service interface {
	IngestSingle(ctx context.Context, userID uuid.UUID, req IngestRequest) (*IngestResponse, *ValidationError, error)
	IngestBatch(ctx context.Context, userID uuid.UUID, req BatchIngestRequest) ([]IngestResponse, *ValidationError, error)
}

type service struct {
	repo Repository
	log  *logger.Logger
	now  func() time.Time
}

func NewService(repo Repository, log *logger.Logger) Service {
	return &service{repo: repo, log: log, now: time.Now}
}

func (s *service) IngestSingle(
	ctx context.Context,
	userID uuid.UUID,
	req IngestRequest,
) (*IngestResponse, *ValidationError, error) {

	rid, _ := reqid.FromContext(ctx)

	s.log.Debug(rid, "Starting IngestSingle for user_id=%s client_event_id=%s", userID, req.ClientEventID)

	if validationErr := ValidateIngest(req, s.now(), ""); validationErr != nil {
		s.log.Error(rid, "Ending IngestSingle for user_id=%s (validation failed: %s on field=%s)", userID, validationErr.Error, validationErr.Field)
		return nil, validationErr, nil
	}

	resp, validationErr, err := s.store(ctx, userID, req)
	if err != nil {
		s.log.Error(rid, "Ending IngestSingle for user_id=%s (failed: %v)", userID, err)
		return nil, nil, err
	}
	if validationErr != nil {
		s.log.Error(rid, "Ending IngestSingle for user_id=%s (validation failed: %s on field=%s)", userID, validationErr.Error, validationErr.Field)
		return nil, validationErr, nil
	}

	s.log.Info(rid, "Ending IngestSingle for user_id=%s (event_id=%s status=%s)", userID, resp.EventID, resp.Status)

	return resp, nil, nil
}

func (s *service) IngestBatch(
	ctx context.Context,
	userID uuid.UUID,
	req BatchIngestRequest,
) ([]IngestResponse, *ValidationError, error) {

	rid, _ := reqid.FromContext(ctx)

	s.log.Debug(rid, "Starting IngestBatch for user_id=%s event_count=%d", userID, len(req.Events))

	// All-or-nothing: validate every event before writing any of them.
	if validationErr := ValidateBatch(req, s.now()); validationErr != nil {
		s.log.Error(rid, "Ending IngestBatch for user_id=%s (validation failed: %s on field=%s)", userID, validationErr.Error, validationErr.Field)
		return nil, validationErr, nil
	}

	results := make([]IngestResponse, 0, len(req.Events))
	for i, event := range req.Events {
		resp, validationErr, err := s.store(ctx, userID, event)
		if err != nil {
			s.log.Error(rid, "Ending IngestBatch for user_id=%s (failed at index=%d: %v)", userID, i, err)
			return nil, nil, err
		}
		if validationErr != nil {
			// Every prior event in this batch was already inserted — batch
			// writes aren't wrapped in one transaction, so this specific
			// DB-detected case (see mapContextCheckViolation) can't fully
			// honor all-or-nothing the way pre-insert validation does.
			// Narrow in practice (only reachable via the jsonb-canonicalization
			// edge case, not a normal oversized-context request, which
			// ValidateBatch already rejects before any writes happen) —
			// still surfaced as a clean validation error rather than a 500.
			validationErr.Field = "events[" + strconv.Itoa(i) + "]." + validationErr.Field
			s.log.Error(rid, "Ending IngestBatch for user_id=%s (validation failed at index=%d: %s)", userID, i, validationErr.Error)
			return nil, validationErr, nil
		}
		results = append(results, *resp)
	}

	s.log.Info(rid, "Ending IngestBatch for user_id=%s (event_count=%d)", userID, len(results))

	return results, nil, nil
}

func (s *service) store(ctx context.Context, userID uuid.UUID, req IngestRequest) (*IngestResponse, *ValidationError, error) {
	event := &Event{
		ID:            uuid.New(),
		ClientEventID: req.ClientEventID,
		UserID:        userID,
		SongID:        req.SongID,
		SessionID:     req.SessionID,
		Type:          req.EventType,
		OccurredAt:    req.OccurredAt,
		PositionMS:    req.PositionMS,
		DurationMS:    req.DurationMS,
		DeviceType:    req.DeviceType,
		Context:       req.Context,
	}

	inserted, err := s.repo.Insert(ctx, event)
	if err != nil {
		if errors.Is(err, ErrContextTooLarge) {
			return nil, &ValidationError{Error: "CONTEXT_TOO_LARGE", Field: "context"}, nil
		}
		return nil, nil, err
	}

	status := IngestStatusDuplicate
	if inserted {
		status = IngestStatusAccepted
	}

	return &IngestResponse{
		EventID:       event.ID,
		ClientEventID: req.ClientEventID,
		Status:        status,
	}, nil, nil
}
