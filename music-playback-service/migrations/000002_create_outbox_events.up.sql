CREATE TABLE outbox_events (
    id UUID PRIMARY KEY,
    playback_event_id UUID NOT NULL REFERENCES playback_events (id),
    event_type TEXT NOT NULL,
    schema_version INTEGER NOT NULL CHECK (schema_version > 0),
    payload JSONB NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at TIMESTAMPTZ,
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    last_error TEXT,
    CONSTRAINT outbox_events_playback_event_unique UNIQUE (playback_event_id),
    CONSTRAINT outbox_events_publish_state CHECK (
        published_at IS NULL OR last_error IS NULL
    )
);

CREATE INDEX outbox_events_unpublished_created_at_idx
    ON outbox_events (created_at)
    WHERE published_at IS NULL;