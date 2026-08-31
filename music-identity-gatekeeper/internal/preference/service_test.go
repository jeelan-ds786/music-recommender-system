package preference

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/logger"
)

type fakeRepository struct {
	pref  *Preference
	items []LikedSong
	next  *Cursor
	err   error
}

func (f *fakeRepository) Create(ctx context.Context, userID uuid.UUID) error {
	return f.err
}

func (f *fakeRepository) GetByUserID(ctx context.Context, userID uuid.UUID) (*Preference, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.pref, nil
}

func (f *fakeRepository) LikeSong(ctx context.Context, userID, songID uuid.UUID) error {
	return f.err
}

func (f *fakeRepository) UnlikeSong(ctx context.Context, userID, songID uuid.UUID) error {
	return f.err
}

func (f *fakeRepository) ListLikedSongs(ctx context.Context, userID uuid.UUID, cursor *Cursor, limit int) ([]LikedSong, *Cursor, error) {
	if f.err != nil {
		return nil, nil, f.err
	}
	return f.items, f.next, nil
}

func (f *fakeRepository) FollowArtist(ctx context.Context, userID, artistID uuid.UUID) error {
	return f.err
}

func (f *fakeRepository) UnfollowArtist(ctx context.Context, userID, artistID uuid.UUID) error {
	return f.err
}

func (f *fakeRepository) CompleteOnboarding(ctx context.Context, userID uuid.UUID, fields OnboardingFields) error {
	return f.err
}

func TestService_GetPreferences(t *testing.T) {
	userID := uuid.New()

	tests := []struct {
		name    string
		repo    *fakeRepository
		wantErr error
	}{
		{
			name: "returns mapped preferences",
			repo: &fakeRepository{
				pref: &Preference{
					UserID:            userID,
					LikedSongIDs:      []uuid.UUID{},
					FollowedArtistIDs: []uuid.UUID{},
					GenreSeeds:        []string{"pop"},
					LanguagePrefs:     []string{"en"},
					UpdatedAt:         time.Now(),
				},
			},
		},
		{
			name:    "propagates not-found error",
			repo:    &fakeRepository{err: ErrPreferenceNotFound},
			wantErr: ErrPreferenceNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService(tt.repo, logger.New(logger.LevelNone), nil)

			got, err := svc.GetPreferences(context.Background(), userID)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(got.GenreSeeds) != 1 || got.GenreSeeds[0] != "pop" {
				t.Errorf("GenreSeeds mismatch: got %v", got.GenreSeeds)
			}

			if len(got.LanguagePrefs) != 1 || got.LanguagePrefs[0] != "en" {
				t.Errorf("LanguagePrefs mismatch: got %v", got.LanguagePrefs)
			}
		})
	}
}

func TestService_LikeSong_PropagatesRepoError(t *testing.T) {
	svc := NewService(&fakeRepository{err: errors.New("db down")}, logger.New(logger.LevelNone), nil)

	err := svc.LikeSong(context.Background(), uuid.New(), uuid.New())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestService_UnlikeSong_PropagatesNotFound(t *testing.T) {
	svc := NewService(&fakeRepository{err: ErrLikeNotFound}, logger.New(logger.LevelNone), nil)

	err := svc.UnlikeSong(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, ErrLikeNotFound) {
		t.Fatalf("expected ErrLikeNotFound, got %v", err)
	}
}

func TestService_FollowArtist_PropagatesRepoError(t *testing.T) {
	svc := NewService(&fakeRepository{err: errors.New("db down")}, logger.New(logger.LevelNone), nil)

	err := svc.FollowArtist(context.Background(), uuid.New(), uuid.New())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestService_UnfollowArtist_PropagatesNotFound(t *testing.T) {
	svc := NewService(&fakeRepository{err: ErrFollowNotFound}, logger.New(logger.LevelNone), nil)

	err := svc.UnfollowArtist(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, ErrFollowNotFound) {
		t.Fatalf("expected ErrFollowNotFound, got %v", err)
	}
}

func TestService_Onboard_PropagatesAlreadyCompleted(t *testing.T) {
	svc := NewService(&fakeRepository{err: ErrOnboardingAlreadyCompleted}, logger.New(logger.LevelNone), nil)

	err := svc.Onboard(context.Background(), uuid.New(), OnboardingRequest{GenreSeeds: []string{"pop"}})
	if !errors.Is(err, ErrOnboardingAlreadyCompleted) {
		t.Fatalf("expected ErrOnboardingAlreadyCompleted, got %v", err)
	}
}

func TestService_ListLikedSongs_InvalidCursorRejectedBeforeRepo(t *testing.T) {
	repo := &fakeRepository{err: errors.New("should not be called")}
	svc := NewService(repo, logger.New(logger.LevelNone), nil)

	_, err := svc.ListLikedSongs(context.Background(), uuid.New(), "not-a-valid-cursor", 20)
	if !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("expected ErrInvalidCursor, got %v", err)
	}
}

func TestService_ListLikedSongs_BuildsNextCursor(t *testing.T) {
	songID := uuid.New()
	createdAt := time.Now().UTC()

	repo := &fakeRepository{
		items: []LikedSong{{SongID: songID, CreatedAt: createdAt}},
		next:  &Cursor{ID: songID, CreatedAt: createdAt},
	}
	svc := NewService(repo, logger.New(logger.LevelNone), nil)

	page, err := svc.ListLikedSongs(context.Background(), uuid.New(), "", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(page.Items) != 1 || page.Items[0].SongID != songID {
		t.Fatalf("unexpected items: %+v", page.Items)
	}

	if page.NextCursor == nil {
		t.Fatal("expected NextCursor to be set")
	}

	decoded, err := DecodeCursor(*page.NextCursor)
	if err != nil {
		t.Fatalf("NextCursor did not decode: %v", err)
	}
	if decoded.ID != songID {
		t.Errorf("decoded cursor ID mismatch: got %v want %v", decoded.ID, songID)
	}
}
