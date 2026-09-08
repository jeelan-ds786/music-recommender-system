DELETE FROM outbox_events
WHERE payload IS NOT NULL;

ALTER TABLE outbox_events
    ALTER COLUMN payload TYPE JSONB USING '{}'::jsonb,
    DROP COLUMN headers,
    DROP COLUMN message_key,
    DROP COLUMN topic;