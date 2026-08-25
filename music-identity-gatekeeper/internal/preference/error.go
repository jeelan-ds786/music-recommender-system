package preference

import "errors"

var ErrPreferenceNotFound = errors.New("preference not found")
var ErrLikeNotFound = errors.New("like not found")
var ErrFollowNotFound = errors.New("follow not found")
var ErrOnboardingAlreadyCompleted = errors.New("onboarding already completed")
var ErrInvalidCursor = errors.New("invalid cursor")
