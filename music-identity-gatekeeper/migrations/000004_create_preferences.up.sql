CREATE TABLE preferences (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    liked_song_ids UUID[] NOT NULL DEFAULT '{}',
    followed_artist_ids UUID[] NOT NULL DEFAULT '{}',
    genre_seeds TEXT[] NOT NULL DEFAULT '{}',
    language_prefs TEXT[] NOT NULL DEFAULT '{}',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_preferences_liked_song_ids ON preferences USING GIN (liked_song_ids);
CREATE INDEX idx_preferences_followed_artist_ids ON preferences USING GIN (followed_artist_ids);
