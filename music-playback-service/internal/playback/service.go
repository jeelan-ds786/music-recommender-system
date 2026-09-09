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

const (
	// defaultSessionEventsLimit/maxSessionEventsLimit mirror
	// music-identity-gatekeeper's preference.ListLikedSongs precedent
	// (20/100).
	defaultSessionEventsLimit = 20
	maxSessionEventsLimit     = 100

	// lateThreshold is a judgment call — no existing precedent for this
	// specific number anywhere in the codebase. Adjust if product
	// guidance says otherwise.
	lateThreshold = 5 * time.Minute
)

type Service interface {
	IngestSingle(ctx context.Context, userID uuid.UUID, req IngestRequest) (*IngestResponse, *ValidationError, error)
	IngestBatch(ctx context.Context, userID uuid.UUID, req BatchIngestRequest) ([]IngestResponse, *ValidationError, error)
	GetSessionEvents(ctx context.Context, userID, sessionID uuid.UUID, cursor string, limit int) (*SessionEventsPage, error)
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

func (s *service) GetSessionEvents(
	ctx context.Context,
	userID, sessionID uuid.UUID,
	cursor string,
	limit int,
) (*SessionEventsPage, error) {

	rid, _ := reqid.FromContext(ctx)

	s.log.Debug(rid, "Starting GetSessionEvents for user_id=%s session_id=%s", userID, sessionID)

	var c *Cursor
	if cursor != "" {
		decoded, err := DecodeCursor(cursor)
		if err != nil {
			s.log.Error(rid, "Ending GetSessionEvents for user_id=%s session_id=%s (invalid cursor %q: %v)", userID, sessionID, cursor, err)
			return nil, err
		}
		c = decoded
	}

	if limit <= 0 {
		limit = defaultSessionEventsLimit
	}
	if limit > maxSessionEventsLimit {
		limit = maxSessionEventsLimit
	}

	events, overview, next, err := s.repo.GetSessionEvents(ctx, userID, sessionID, c, limit)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			s.log.Error(rid, "Ending GetSessionEvents for user_id=%s session_id=%s (not found)", userID, sessionID)
		} else {
			s.log.Error(rid, "Ending GetSessionEvents for user_id=%s session_id=%s (failed: %v)", userID, sessionID, err)
		}
		return nil, err
	}

	items := make([]SessionEventResponse, 0, len(events))
	for _, e := range events {
		items = append(items, SessionEventResponse{
			EventID:       e.ID,
			ClientEventID: e.ClientEventID,
			SongID:        e.SongID,
			EventType:     e.Type,
			OccurredAt:    e.OccurredAt,
			IngestedAt:    e.IngestedAt,
			PositionMS:    e.PositionMS,
			DurationMS:    e.DurationMS,
			DeviceType:    e.DeviceType,
			Quality:       computeQuality(e, overview.OutOfOrder[e.ID]),
		})
	}

	page := &SessionEventsPage{
		Session: SessionSummaryResponse{
			SessionID:  sessionID,
			EventCount: overview.EventCount,
			StartedAt:  overview.StartedAt,
			EndedAt:    overview.EndedAt,
			DurationMS: overview.DurationMS,
		},
		Events: items,
	}
	if next != nil {
		encoded := EncodeCursor(*next)
		page.NextCursor = &encoded
	}

	s.log.Info(rid, "Ending GetSessionEvents for user_id=%s session_id=%s (event_count=%d)", userID, sessionID, len(items))

	return page, nil
}

// computeQuality derives read-time-only flags from an event's own facts.
// Unlike OutOfOrder (session-wide, computed by the repository), Late and
// CompletionPercentage only need the event's own fields.
func computeQuality(e Event, outOfOrder bool) EventQuality {
	quality := EventQuality{
		OutOfOrder: outOfOrder,
		Late:       e.IngestedAt.Sub(e.OccurredAt) > lateThreshold,
	}

	if e.DurationMS != nil && *e.DurationMS > 0 {
		pct := float64(e.PositionMS) / float64(*e.DurationMS)
		switch {
		case pct < 0:
			pct = 0
		case pct > 1:
			pct = 1
		}
		quality.CompletionPercentage = &pct
	}

	return quality
}
