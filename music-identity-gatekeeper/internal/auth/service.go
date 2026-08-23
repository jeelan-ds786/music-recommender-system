package auth

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

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
}

func NewService(
	userRepo user.Repository,
	tokenService *token.Service,
) Service {
	return &AuthService{
		userRepo:        userRepo,
		tokenService:    tokenService,
		comparePassword: ComparePassword,
	}
}

func (s *AuthService) Register(
	ctx context.Context,
	req RegisterRequest,
) (*RegisterResponse, error) {
	req.Email = normalizeEmail(req.Email)

	existingUser, err := s.userRepo.GetByEmail(
		ctx,
		req.Email,
	)

	if err == nil && existingUser != nil {
		return nil, ErrEmailAlreadyExists
	}

	if err != nil &&
		!errors.Is(err, user.ErrUserNotFound) {
		return nil, err
	}

	passwordHash, err := HashPassword(req.Password)
	if err != nil {
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
		return nil, err
	}

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
		existingUser.AuthProvider,
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
