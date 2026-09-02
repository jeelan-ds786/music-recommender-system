package playlist

import "errors"

var ErrPlaylistNotFound = errors.New("playlist not found")
var ErrSongNotInPlaylist = errors.New("song not in playlist")
