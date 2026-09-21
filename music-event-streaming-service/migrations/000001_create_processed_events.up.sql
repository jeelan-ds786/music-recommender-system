CREATE TABLE processed_events (
    event_id TEXT NOT NULL,
    handler_name TEXT NOT NULL,
    state TEXT NOT NULL,
    lease_until TIMESTAMPTZ,
    processed_at TIMESTAMPTZ,
    PRIMARY KEY (event_id, handler_name),
    CHECK (event_id <> ''),
    CHECK (handler_name <> ''),
    CHECK (state IN ('processing', 'processed')),
    CHECK (
        (state = 'processing' AND lease_until IS NOT NULL AND processed_at IS NULL)
        OR (state = 'processed' AND lease_until IS NULL AND processed_at IS NOT NULL)
    )
);

CREATE INDEX processed_events_processed_at_idx
    ON processed_events (processed_at);