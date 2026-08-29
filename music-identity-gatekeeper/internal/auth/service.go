package auth

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/event"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/logger"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/reqid"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/token"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/user"
)

const dummyPasswordHash = "$2a$12$D4G5f18o7aMMfwasBL7X6uJkHQdVyCzEOv.a8cF9Q8S1cS1Jyd9rq"

type Service interface {
	Register(ctx context.Context, req RegisterRequest) (*RegisterResponse, error)
	Login(ctx context.Context, req LoginRequest) (*token.TokenPair, error)
	Refresh(ctx context.Context, req RefreshRequest) (*token.TokenPair, error)
}

type AuthService struct {
	userRepo        user.Repository
	tokenService    *token.Service
	comparePassword func(string, string) error
	log             *logger.Logger
	emitter         *event.Emitter
}

func NewService(
	userRepo user.Repository,
	tokenService *token.Service,
	log *logger.Logger,
	emitter *event.Emitter,
) Service {
	return &AuthService{
		userRepo:        userRepo,
		tokenService:    tokenService,
		comparePassword: ComparePassword,
		log:             log,
		emitter:         emitter,
	}
}

func (s *AuthService) Register(
	ctx context.Context,
	req RegisterRequest,
) (*RegisterResponse, error) {
	req.Email = normalizeEmail(req.Email)

	rid, _ := reqid.FromContext(ctx)

	s.log.Debug(rid, "Starting registration for email=%s", req.Email)

	s.log.Debug(rid, "checking users table for email=%s", req.Email)

	existingUser, err := s.userRepo.GetByEmail(
		ctx,
		req.Email,
	)

	if err == nil && existingUser != nil {
		s.log.Info(rid, "found existing user id=%s for email=%s", existingUser.ID, req.Email)
		s.log.Info(rid, "Ending registration for email=%s (rejected: already exists)", req.Email)
		return nil, ErrEmailAlreadyExists
	}

	if err != nil &&
		!errors.Is(err, user.ErrUserNotFound) {
		s.log.Error(rid, "Ending registration for email=%s (lookup error: %v)", req.Email, err)
		return nil, err
	}

	s.log.Debug(rid, "no data found for email=%s", req.Email)

	passwordHash, err := HashPassword(req.Password)
	if err != nil {
		s.log.Error(rid, "Ending registration for email=%s (hash error: %v)", req.Email, err)
		return nil, err
	}

	newUser := &user.User{
		ID:             uuid.New(),
		Email:          req.Email,
		HashedPassword: passwordHash,
		AuthProvider:   "local",
	}

	err = s.userRepo.CreateUser(
		ctx,
		newUser,
	)

	if err != nil {
		s.log.Error(rid, "Ending registration for email=%s (write error: %v)", req.Email, err)
		return nil, err
	}

	s.log.Info(rid, "write completed for user id=%s email=%s", newUser.ID, req.Email)

	if s.emitter != nil {
		if err := s.emitter.EmitUserRegistered(ctx, nil, newUser.ID.String(), req.Email); err != nil {
			s.log.Error(rid, "failed to enqueue user.registered event for user_id=%s: %v", newUser.ID, err)
		}
	}

	s.log.Debug(rid, "Ending registration for email=%s", req.Email)

	return &RegisterResponse{
		ID:      newUser.ID,
		Email:   newUser.Email,
		Message: "user registered successfully",
	}, nil
}

func (s *AuthService) Login(
	ctx context.Context,
	req LoginRequest,
) (*token.TokenPair, error) {
	req.Email = normalizeEmail(req.Email)

	existingUser, err := s.userRepo.GetByEmail(
		ctx,
		req.Email,
	)

	if err != nil && !errors.Is(err, user.ErrUserNotFound) {
		return nil, err
	}

	passwordHash := dummyPasswordHash
	if err == nil && existingUser != nil {
		passwordHash = existingUser.HashedPassword
	}

	passwordErr := s.comparePassword(passwordHash, req.Password)
	if passwordErr != nil || existingUser == nil {
		return nil, ErrInvalidCredentials
	}

	return s.tokenService.IssueTokenPair(
		ctx,
		existingUser.ID,
	)
}

func (s *AuthService) Refresh(
	ctx context.Context,
	req RefreshRequest,
) (*token.TokenPair, error) {
	return s.tokenService.RefreshAccessToken(
		ctx,
		req.RefreshToken,
	)
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
