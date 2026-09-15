ALTER TABLE outbox_events
    ADD COLUMN topic TEXT NOT NULL DEFAULT 'playback.event.v1',
    ADD COLUMN message_key TEXT,
    ADD COLUMN headers JSONB;

UPDATE outbox_events AS outbox
SET message_key = playback.user_id::text,
    headers = jsonb_build_object(
        'content-type', 'application/x-protobuf',
        'event-type', outbox.event_type,
        'schema-version', outbox.schema_version::text
    )
FROM playback_events AS playback
WHERE playback.id = outbox.playback_event_id;

ALTER TABLE outbox_events
    ALTER COLUMN topic DROP DEFAULT,
    ALTER COLUMN message_key SET NOT NULL,
    ALTER COLUMN headers SET NOT NULL,
    ALTER COLUMN payload TYPE BYTEA USING convert_to(payload::text, 'UTF8');