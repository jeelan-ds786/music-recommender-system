package oauth

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/token"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/user"
)

const googleProvider = "google"

var (
	ErrMissingCode     = errors.New("missing oauth code")
	ErrEmailUnverified = errors.New("oauth email is not verified")
	ErrEmailConflict   = errors.New("email belongs to another authentication provider")
	ErrProviderFailure = errors.New("oauth provider failure")
)

type UserRepository interface {
	GetByEmail(ctx context.Context, email string) (*user.User, error)
}

type AccountRepository interface {
	GetUser(ctx context.Context, provider, providerSubject string) (*user.User, error)
	CreateUser(ctx context.Context, user *user.User, provider, providerSubject string) error
}

type ProfileInitializer interface {
	EnsureExists(ctx context.Context, userID uuid.UUID) error
}

type TokenIssuer interface {
	IssueTokenPair(ctx context.Context, userID uuid.UUID) (*token.TokenPair, error)
}

type Service struct {
	states   *StateManager
	provider Provider
	users    UserRepository
	accounts AccountRepository
	profiles ProfileInitializer
	tokens   TokenIssuer
}

func NewService(
	states *StateManager,
	provider Provider,
	users UserRepository,
	accounts AccountRepository,
	profiles ProfileInitializer,
	tokens TokenIssuer,
) *Service {
	return &Service{states: states, provider: provider, users: users, accounts: accounts, profiles: profiles, tokens: tokens}
}

func (s *Service) Begin(ctx context.Context) (string, error) {
	state, err := s.states.Create(ctx)
	if err != nil {
		return "", err
	}
	return s.provider.AuthorizationURL(state), nil
}

func (s *Service) Callback(ctx context.Context, code, state string) (*token.TokenPair, error) {
	if code == "" {
		return nil, ErrMissingCode
	}
	if err := s.states.Validate(ctx, state); err != nil {
		return nil, err
	}

	identity, err := s.provider.Exchange(ctx, code)
	if err != nil {
		return nil, errors.Join(ErrProviderFailure, err)
	}
	if !identity.EmailVerified || identity.Email == "" || identity.Subject == "" {
		return nil, ErrEmailUnverified
	}

	email := strings.ToLower(strings.TrimSpace(identity.Email))
	linkedUser, err := s.accounts.GetUser(ctx, googleProvider, identity.Subject)
	if err == nil {
		return s.tokens.IssueTokenPair(ctx, linkedUser.ID)
	}
	if !errors.Is(err, user.ErrUserNotFound) {
		return nil, err
	}

	_, err = s.users.GetByEmail(ctx, email)
	if err == nil {
		return nil, ErrEmailConflict
	}
	if !errors.Is(err, user.ErrUserNotFound) {
		return nil, err
	}

	newUser := &user.User{ID: uuid.New(), Email: email, AuthProvider: googleProvider}
	if err := s.accounts.CreateUser(ctx, newUser, googleProvider, identity.Subject); err != nil {
		return nil, err
	}
	if err := s.profiles.EnsureExists(ctx, newUser.ID); err != nil {
		return nil, err
	}
	return s.tokens.IssueTokenPair(ctx, newUser.ID)
}
