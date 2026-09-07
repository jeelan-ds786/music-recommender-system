CREATE TABLE playback_events (
    id UUID PRIMARY KEY,
    client_event_id UUID NOT NULL,
    user_id UUID NOT NULL,
    song_id UUID NOT NULL,
    session_id UUID NOT NULL,
    event_type TEXT NOT NULL CHECK (
        event_type IN ('play', 'pause', 'seek', 'skip', 'replay', 'complete')
    ),
    occurred_at TIMESTAMPTZ NOT NULL,
    position_ms BIGINT NOT NULL CHECK (position_ms >= 0),
    duration_ms BIGINT CHECK (duration_ms IS NULL OR duration_ms >= 0),
    device_type TEXT NOT NULL CHECK (length(trim(device_type)) > 0),
    context JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (octet_length(context::text) <= 4096),
    ingested_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT playback_events_user_client_event_unique UNIQUE (user_id, client_event_id)
);

CREATE INDEX playback_events_user_occurred_at_idx
    ON playback_events (user_id, occurred_at);

CREATE INDEX playback_events_session_occurred_at_idx
    ON playback_events (session_id, occurred_at);

CREATE FUNCTION reject_playback_event_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'playback_events is append-only';
END;
$$;

CREATE TRIGGER playback_events_append_only
BEFORE UPDATE OR DELETE ON playback_events
FOR EACH ROW EXECUTE FUNCTION reject_playback_event_mutation();