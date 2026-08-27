package oauth

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/token"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/user"
)

type fakeProvider struct {
	identity      *Identity
	err           error
	exchangeCalls int
}

func (p *fakeProvider) AuthorizationURL(state string) string {
	return "https://accounts.example/authorize?state=" + state
}

func (p *fakeProvider) Exchange(context.Context, string) (*Identity, error) {
	p.exchangeCalls++
	return p.identity, p.err
}

type fakeUserRepository struct {
	existing  *user.User
	lookupErr error
}

func (r *fakeUserRepository) GetByEmail(context.Context, string) (*user.User, error) {
	return r.existing, r.lookupErr
}

type fakeAccountRepository struct {
	existing  *user.User
	lookupErr error
	created   *user.User
	provider  string
	subject   string
}

func (r *fakeAccountRepository) GetUser(context.Context, string, string) (*user.User, error) {
	return r.existing, r.lookupErr
}

func (r *fakeAccountRepository) CreateUser(
	_ context.Context,
	created *user.User,
	provider string,
	subject string,
) error {
	r.created = created
	r.provider = provider
	r.subject = subject
	return nil
}

type fakeProfileInitializer struct {
	userID uuid.UUID
}

func (p *fakeProfileInitializer) EnsureExists(_ context.Context, userID uuid.UUID) error {
	p.userID = userID
	return nil
}

type fakeTokenIssuer struct {
	userID uuid.UUID
}

func (i *fakeTokenIssuer) IssueTokenPair(_ context.Context, userID uuid.UUID) (*token.TokenPair, error) {
	i.userID = userID
	return &token.TokenPair{AccessToken: "access", RefreshToken: "refresh"}, nil
}

func newOAuthTestService(
	provider *fakeProvider,
	users *fakeUserRepository,
	profiles *fakeProfileInitializer,
	tokens *fakeTokenIssuer,
	accountRepositories ...*fakeAccountRepository,
) (*Service, *StateManager) {
	states := NewStateManager(newMemoryStateStore())
	accounts := &fakeAccountRepository{lookupErr: user.ErrUserNotFound}
	if len(accountRepositories) > 0 {
		accounts = accountRepositories[0]
	}
	return NewService(states, provider, users, accounts, profiles, tokens), states
}

func validState(t *testing.T, states *StateManager) string {
	t.Helper()
	state, err := states.Create(context.Background())
	if err != nil {
		t.Fatalf("Create() state error = %v", err)
	}
	return state
}

func TestCallbackRejectsInvalidStateBeforeProviderExchange(t *testing.T) {
	provider := &fakeProvider{}
	service, _ := newOAuthTestService(provider, &fakeUserRepository{}, &fakeProfileInitializer{}, &fakeTokenIssuer{})

	for _, state := range []string{"", "unknown"} {
		if _, err := service.Callback(context.Background(), "code", state); !errors.Is(err, ErrInvalidState) {
			t.Fatalf("Callback() state %q error = %v, want ErrInvalidState", state, err)
		}
	}
	if provider.exchangeCalls != 0 {
		t.Fatalf("provider exchange calls = %d, want 0", provider.exchangeCalls)
	}
}

func TestCallbackConsumesStateOnce(t *testing.T) {
	provider := &fakeProvider{identity: &Identity{Subject: "google-sub", Email: "listener@example.com", EmailVerified: true}}
	users := &fakeUserRepository{lookupErr: user.ErrUserNotFound}
	service, states := newOAuthTestService(provider, users, &fakeProfileInitializer{}, &fakeTokenIssuer{})
	state := validState(t, states)

	if _, err := service.Callback(context.Background(), "code", state); err != nil {
		t.Fatalf("Callback() error = %v", err)
	}
	if _, err := service.Callback(context.Background(), "code", state); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("reused state error = %v, want ErrInvalidState", err)
	}
	if provider.exchangeCalls != 1 {
		t.Fatalf("provider exchange calls = %d, want 1", provider.exchangeCalls)
	}
}

func TestCallbackRejectsUnverifiedEmail(t *testing.T) {
	provider := &fakeProvider{identity: &Identity{Subject: "google-sub", Email: "listener@example.com"}}
	service, states := newOAuthTestService(provider, &fakeUserRepository{}, &fakeProfileInitializer{}, &fakeTokenIssuer{})

	_, err := service.Callback(context.Background(), "code", validState(t, states))
	if !errors.Is(err, ErrEmailUnverified) {
		t.Fatalf("Callback() error = %v, want ErrEmailUnverified", err)
	}
}

func TestCallbackDoesNotSilentlyLinkLocalAccount(t *testing.T) {
	provider := &fakeProvider{identity: &Identity{Subject: "google-sub", Email: "listener@example.com", EmailVerified: true}}
	users := &fakeUserRepository{existing: &user.User{ID: uuid.New(), Email: "listener@example.com", AuthProvider: "local"}}
	accounts := &fakeAccountRepository{lookupErr: user.ErrUserNotFound}
	service, states := newOAuthTestService(provider, users, &fakeProfileInitializer{}, &fakeTokenIssuer{}, accounts)

	_, err := service.Callback(context.Background(), "code", validState(t, states))
	if !errors.Is(err, ErrEmailConflict) {
		t.Fatalf("Callback() error = %v, want ErrEmailConflict", err)
	}
	if accounts.created != nil {
		t.Fatal("local account was modified or replaced")
	}
}

func TestCallbackCreatesOAuthUserWithoutPasswordAndIssuesTokenPair(t *testing.T) {
	provider := &fakeProvider{identity: &Identity{Subject: "google-sub", Email: " LISTENER@Example.COM ", EmailVerified: true}}
	users := &fakeUserRepository{lookupErr: user.ErrUserNotFound}
	profiles := &fakeProfileInitializer{}
	tokens := &fakeTokenIssuer{}
	accounts := &fakeAccountRepository{lookupErr: user.ErrUserNotFound}
	service, states := newOAuthTestService(provider, users, profiles, tokens, accounts)

	pair, err := service.Callback(context.Background(), "code", validState(t, states))
	if err != nil {
		t.Fatalf("Callback() error = %v", err)
	}
	if pair.AccessToken != "access" || pair.RefreshToken != "refresh" {
		t.Fatalf("token pair = %#v", pair)
	}
	if accounts.created == nil || accounts.created.Email != "listener@example.com" || accounts.created.AuthProvider != googleProvider {
		t.Fatalf("created user = %#v", accounts.created)
	}
	if accounts.created.HashedPassword != "" {
		t.Fatal("OAuth-only user received a password hash")
	}
	if accounts.provider != googleProvider || accounts.subject != "google-sub" {
		t.Fatalf("provider account = %q/%q", accounts.provider, accounts.subject)
	}
	if profiles.userID != accounts.created.ID || tokens.userID != accounts.created.ID {
		t.Fatal("profile initialization or token issuance used the wrong user")
	}
}

func TestCallbackSignsInExistingGoogleAccount(t *testing.T) {
	existingUser := &user.User{ID: uuid.New(), Email: "listener@example.com", AuthProvider: googleProvider}
	provider := &fakeProvider{identity: &Identity{Subject: "google-sub", Email: existingUser.Email, EmailVerified: true}}
	tokens := &fakeTokenIssuer{}
	accounts := &fakeAccountRepository{existing: existingUser}
	service, states := newOAuthTestService(provider, &fakeUserRepository{}, &fakeProfileInitializer{}, tokens, accounts)

	if _, err := service.Callback(context.Background(), "code", validState(t, states)); err != nil {
		t.Fatalf("Callback() error = %v", err)
	}
	if tokens.userID != existingUser.ID {
		t.Fatalf("token user = %s, want %s", tokens.userID, existingUser.ID)
	}
}

func TestBeginReturnsAuthorizationURLWithPersistedState(t *testing.T) {
	service, _ := newOAuthTestService(&fakeProvider{}, &fakeUserRepository{}, &fakeProfileInitializer{}, &fakeTokenIssuer{})
	url, err := service.Begin(context.Background())
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	if !strings.HasPrefix(url, "https://accounts.example/authorize?state=") {
		t.Fatalf("authorization URL = %q", url)
	}
}
