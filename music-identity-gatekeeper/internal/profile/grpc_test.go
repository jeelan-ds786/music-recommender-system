package profile

import (
	"context"
	"errors"
	"net"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/profile/profilepb"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/user"
)

type grpcProfileService struct {
	response *MeResponse
	err      error
}

func (s *grpcProfileService) GetMe(context.Context, uuid.UUID) (*MeResponse, error) {
	return s.response, s.err
}

func (s *grpcProfileService) PatchMe(context.Context, uuid.UUID, PatchMeRequest) (*MeResponse, error) {
	return nil, nil
}

func newTestIdentityClient(t *testing.T, profiles Service) profilepb.IdentityServiceClient {
	t.Helper()

	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	profilepb.RegisterIdentityServiceServer(server, NewGRPCServer(profiles))
	go func() {
		if err := server.Serve(listener); err != nil {
			t.Errorf("Serve() error = %v", err)
		}
	}()
	t.Cleanup(server.Stop)

	connection, err := grpc.NewClient(
		"passthrough:///bufconn",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return listener.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	return profilepb.NewIdentityServiceClient(connection)
}

func TestIdentityServiceClientGetsListenerProfile(t *testing.T) {
	userID := uuid.New()
	followedArtistIDs := []uuid.UUID{uuid.New(), uuid.New()}
	client := newTestIdentityClient(t, &grpcProfileService{response: &MeResponse{
		Account: AccountFields{ID: userID},
		Tier:    TierPremium,
		Preferences: PreferenceFields{
			LikedSongIDs:      []uuid.UUID{uuid.New(), uuid.New(), uuid.New()},
			FollowedArtistIDs: followedArtistIDs,
			GenreSeeds:        []string{"jazz", "soul"},
			LanguagePrefs:     []string{"en", "fr"},
		},
	}})

	response, err := client.GetListenerProfile(context.Background(), &profilepb.GetListenerProfileRequest{UserId: userID.String()})
	if err != nil {
		t.Fatalf("GetListenerProfile() error = %v", err)
	}
	wantArtists := []string{followedArtistIDs[0].String(), followedArtistIDs[1].String()}
	if response.GetUserId() != userID.String() || response.GetTier() != TierPremium || response.GetLikedSongCount() != 3 {
		t.Fatalf("response = %#v", response)
	}
	if !reflect.DeepEqual(response.GetGenreSeeds(), []string{"jazz", "soul"}) ||
		!reflect.DeepEqual(response.GetLanguagePreferences(), []string{"en", "fr"}) ||
		!reflect.DeepEqual(response.GetFollowedArtistIds(), wantArtists) {
		t.Fatalf("response preferences = %#v", response)
	}
}

func TestIdentityServiceClientMapsErrors(t *testing.T) {
	t.Run("invalid UUID", func(t *testing.T) {
		client := newTestIdentityClient(t, &grpcProfileService{})
		_, err := client.GetListenerProfile(context.Background(), &profilepb.GetListenerProfileRequest{UserId: "invalid"})
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("status code = %s, want %s", status.Code(err), codes.InvalidArgument)
		}
	})

	t.Run("missing user", func(t *testing.T) {
		client := newTestIdentityClient(t, &grpcProfileService{err: user.ErrUserNotFound})
		_, err := client.GetListenerProfile(context.Background(), &profilepb.GetListenerProfileRequest{UserId: uuid.NewString()})
		if status.Code(err) != codes.NotFound {
			t.Fatalf("status code = %s, want %s", status.Code(err), codes.NotFound)
		}
	})

	t.Run("service error", func(t *testing.T) {
		client := newTestIdentityClient(t, &grpcProfileService{err: errors.New("database unavailable")})
		_, err := client.GetListenerProfile(context.Background(), &profilepb.GetListenerProfileRequest{UserId: uuid.NewString()})
		if status.Code(err) != codes.Internal {
			t.Fatalf("status code = %s, want %s", status.Code(err), codes.Internal)
		}
	})
}
