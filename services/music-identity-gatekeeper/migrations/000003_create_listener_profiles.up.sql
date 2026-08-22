CREATE TYPE subscription_tier AS ENUM (
    'free',
    'premium',
    'family',
    'student',
    'duo'
);

CREATE TABLE listener_profiles (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    display_name VARCHAR(100),
    avatar_url TEXT,
    country CHAR(2),
    language VARCHAR(10),
    birth_year SMALLINT,
    subscription_tier subscription_tier NOT NULL DEFAULT 'free',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);