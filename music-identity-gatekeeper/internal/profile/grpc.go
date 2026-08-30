package profile

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/profile/profilepb"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/user"
)

type GRPCServer struct {
	profilepb.UnimplementedIdentityServiceServer
	profiles Service
}

func NewGRPCServer(profiles Service) *GRPCServer {
	return &GRPCServer{profiles: profiles}
}

func (s *GRPCServer) GetListenerProfile(
	ctx context.Context,
	request *profilepb.GetListenerProfileRequest,
) (*profilepb.GetListenerProfileResponse, error) {
	userID, err := uuid.Parse(request.GetUserId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "user_id must be a valid UUID")
	}

	listenerProfile, err := s.profiles.GetMe(ctx, userID)
	if errors.Is(err, user.ErrUserNotFound) {
		return nil, status.Error(codes.NotFound, "listener not found")
	}
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to load listener profile")
	}

	followedArtistIDs := make([]string, len(listenerProfile.Preferences.FollowedArtistIDs))
	for index, artistID := range listenerProfile.Preferences.FollowedArtistIDs {
		followedArtistIDs[index] = artistID.String()
	}

	return &profilepb.GetListenerProfileResponse{
		UserId:              listenerProfile.Account.ID.String(),
		Tier:                listenerProfile.Tier,
		GenreSeeds:          listenerProfile.Preferences.GenreSeeds,
		LanguagePreferences: listenerProfile.Preferences.LanguagePrefs,
		FollowedArtistIds:   followedArtistIDs,
		LikedSongCount:      int32(len(listenerProfile.Preferences.LikedSongIDs)),
	}, nil
}
