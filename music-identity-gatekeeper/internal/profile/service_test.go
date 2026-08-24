package profile

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/logger"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/preference"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/user"
)

type fakeProfileRepository struct {
	profile      *Profile
	getErr       error
	ensureCalled bool
	ensureErr    error
	updatePatch  PatchFields
	updateResult *Profile
	updateErr    error
}

func (f *fakeProfileRepository) GetByUserID(ctx context.Context, userID uuid.UUID) (*Profile, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.profile, nil
}

func (f *fakeProfileRepository) EnsureExists(ctx context.Context, userID uuid.UUID) error {
	f.ensureCalled = true
	if f.ensureErr != nil {
		return f.ensureErr
	}
	// Once ensured, subsequent reads should succeed.
	f.getErr = nil
	if f.profile == nil {
		f.profile = &Profile{UserID: userID, SubscriptionTier: "free"}
	}
	return nil
}

func (f *fakeProfileRepository) Update(ctx context.Context, userID uuid.UUID, patch PatchFields) (*Profile, error) {
	f.updatePatch = patch
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	return f.updateResult, nil
}

type fakePreferenceRepository struct {
	pref   *preference.Preference
	getErr error
}

func (f *fakePreferenceRepository) Create(ctx context.Context, userID uuid.UUID) error {
	return nil
}

func (f *fakePreferenceRepository) GetByUserID(ctx context.Context, userID uuid.UUID) (*preference.Preference, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.pref, nil
}

type fakeUserRepository struct {
	u      *user.User
	getErr error
}

func (f *fakeUserRepository) CreateUser(ctx context.Context, u *user.User) error {
	return nil
}

func (f *fakeUserRepository) GetByEmail(ctx context.Context, email string) (*user.User, error) {
	return f.u, f.getErr
}

func (f *fakeUserRepository) GetByID(ctx context.Context, id uuid.UUID) (*user.User, error) {
	return f.u, f.getErr
}

func TestService_GetMe(t *testing.T) {
	userID := uuid.New()

	baseUser := &user.User{ID: userID, Email: "test@example.com", CreatedAt: time.Now()}
	basePref := &preference.Preference{
		UserID:            userID,
		LikedSongIDs:      []uuid.UUID{},
		FollowedArtistIDs: []uuid.UUID{},
		GenreSeeds:        []string{},
		LanguagePrefs:     []string{},
	}
	baseProfile := &Profile{UserID: userID, SubscriptionTier: "free"}

	t.Run("existing rows are returned without EnsureExists", func(t *testing.T) {
		profileRepo := &fakeProfileRepository{profile: baseProfile}
		prefRepo := &fakePreferenceRepository{pref: basePref}
		userRepo := &fakeUserRepository{u: baseUser}

		svc := NewService(profileRepo, prefRepo, userRepo, logger.New(logger.LevelNone))

		resp, err := svc.GetMe(context.Background(), userID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Tier != "free" {
			t.Errorf("expected tier free, got %s", resp.Tier)
		}
		if profileRepo.ensureCalled {
			t.Error("EnsureExists should not be called when rows already exist")
		}
	})

	t.Run("first-touch triggers EnsureExists then succeeds", func(t *testing.T) {
		profileRepo := &fakeProfileRepository{getErr: ErrProfileNotFound}
		prefRepo := &fakePreferenceRepository{pref: basePref}
		userRepo := &fakeUserRepository{u: baseUser}

		svc := NewService(profileRepo, prefRepo, userRepo, logger.New(logger.LevelNone))

		resp, err := svc.GetMe(context.Background(), userID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !profileRepo.ensureCalled {
			t.Error("expected EnsureExists to be called on first touch")
		}
		if resp == nil {
			t.Fatal("expected a response")
		}
	})

	t.Run("EnsureExists error propagates", func(t *testing.T) {
		wantErr := errors.New("db down")
		profileRepo := &fakeProfileRepository{getErr: ErrProfileNotFound, ensureErr: wantErr}
		prefRepo := &fakePreferenceRepository{pref: basePref}
		userRepo := &fakeUserRepository{u: baseUser}

		svc := NewService(profileRepo, prefRepo, userRepo, logger.New(logger.LevelNone))

		_, err := svc.GetMe(context.Background(), userID)
		if !errors.Is(err, wantErr) {
			t.Fatalf("expected %v, got %v", wantErr, err)
		}
	})

	t.Run("deleted user propagates user.ErrUserNotFound", func(t *testing.T) {
		profileRepo := &fakeProfileRepository{profile: baseProfile}
		prefRepo := &fakePreferenceRepository{pref: basePref}
		userRepo := &fakeUserRepository{getErr: user.ErrUserNotFound}

		svc := NewService(profileRepo, prefRepo, userRepo, logger.New(logger.LevelNone))

		_, err := svc.GetMe(context.Background(), userID)
		if !errors.Is(err, user.ErrUserNotFound) {
			t.Fatalf("expected user.ErrUserNotFound, got %v", err)
		}
	})
}

func TestService_PatchMe(t *testing.T) {
	userID := uuid.New()
	baseUser := &user.User{ID: userID, Email: "test@example.com", CreatedAt: time.Now()}
	basePref := &preference.Preference{UserID: userID}

	t.Run("single field patch reaches repo with only that field set", func(t *testing.T) {
		updated := &Profile{UserID: userID, SubscriptionTier: "free"}
		profileRepo := &fakeProfileRepository{profile: &Profile{UserID: userID}, updateResult: updated}
		prefRepo := &fakePreferenceRepository{pref: basePref}
		userRepo := &fakeUserRepository{u: baseUser}

		svc := NewService(profileRepo, prefRepo, userRepo, logger.New(logger.LevelNone))

		name := "New Name"
		_, err := svc.PatchMe(context.Background(), userID, PatchMeRequest{DisplayName: &name})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if profileRepo.updatePatch.DisplayName == nil || *profileRepo.updatePatch.DisplayName != name {
			t.Errorf("expected DisplayName patch to be %q, got %v", name, profileRepo.updatePatch.DisplayName)
		}
		if profileRepo.updatePatch.Country != nil {
			t.Errorf("expected Country to remain nil (omitted), got %v", profileRepo.updatePatch.Country)
		}
	})

	t.Run("first-touch-then-patch ensures rows exist first", func(t *testing.T) {
		updated := &Profile{UserID: userID, SubscriptionTier: "free"}
		profileRepo := &fakeProfileRepository{getErr: ErrProfileNotFound, updateResult: updated}
		prefRepo := &fakePreferenceRepository{pref: basePref}
		userRepo := &fakeUserRepository{u: baseUser}

		svc := NewService(profileRepo, prefRepo, userRepo, logger.New(logger.LevelNone))

		name := "New Name"
		_, err := svc.PatchMe(context.Background(), userID, PatchMeRequest{DisplayName: &name})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !profileRepo.ensureCalled {
			t.Error("expected EnsureExists to be called for a never-initialized profile")
		}
	})
}
