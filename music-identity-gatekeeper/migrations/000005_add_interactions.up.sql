CREATE TABLE liked_songs (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    song_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, song_id)
);

CREATE INDEX idx_liked_songs_user_cursor
    ON liked_songs (user_id, created_at DESC, song_id DESC);

CREATE TABLE followed_artists (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    artist_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, artist_id)
);

CREATE INDEX idx_followed_artists_user ON followed_artists (user_id);

ALTER TABLE preferences ADD COLUMN onboarded_at TIMESTAMPTZ;
