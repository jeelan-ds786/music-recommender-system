package preference

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

type fakeRepository struct {
	pref *Preference
	err  error
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
			svc := NewService(tt.repo)

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
