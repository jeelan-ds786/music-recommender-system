CREATE TABLE IF NOT EXISTS kafka_integration (
    id            BIGSERIAL    PRIMARY KEY,
    topic         TEXT         NOT NULL,
    key           TEXT         NOT NULL,
    payload       JSONB        NOT NULL,
    status        TEXT         NOT NULL DEFAULT 'pending',
    attempts      INT          NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    published_at  TIMESTAMPTZ
);

CREATE INDEX idx_kafka_integration_pending
    ON kafka_integration (created_at)
    WHERE status = 'pending';
