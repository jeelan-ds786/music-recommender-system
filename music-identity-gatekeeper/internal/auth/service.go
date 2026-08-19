package auth

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/token"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/user"
)

type Service interface {
	Register(ctx context.Context, req RegisterRequest) (*RegisterResponse, error)
	Login(ctx context.Context, req LoginRequest) (*token.TokenPair, error)
	Refresh(ctx context.Context, req RefreshRequest) (*token.TokenPair, error)
}

type AuthService struct {
	userRepo      user.Repository
	tokenService *token.Service
}

func NewService(
	userRepo user.Repository,
	tokenService *token.Service,
) Service {
	return &AuthService{
		userRepo:      userRepo,
		tokenService: tokenService,
	}
}

func (s *AuthService) Register(
	ctx context.Context,
	req RegisterRequest,
) (*RegisterResponse, error) {

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
		ID:           uuid.New(),
		Email:        req.Email,
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

	existingUser, err := s.userRepo.GetByEmail(
		ctx,
		req.Email,
	)

	if err != nil {
		return nil, ErrInvalidCredentials
	}

	err = ComparePassword(
		existingUser.HashedPassword,
		req.Password,
	)

	if err != nil {
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