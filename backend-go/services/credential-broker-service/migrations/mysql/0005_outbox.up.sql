-- MySQL/TiDB variant of the Postgres outbox (JSON instead of JSONB; the relay
-- only reads payload whole). See migrations/postgres/0005_outbox.up.sql.
CREATE TABLE outbox_events (
    id            CHAR(36) PRIMARY KEY,
    tenant_id     CHAR(36) NOT NULL,
    subject       VARCHAR(255) NOT NULL,
    occurred_at   TIMESTAMP(6) NOT NULL,
    version       INT NOT NULL,
    payload       JSON NOT NULL,
    created_at    TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    published_at  TIMESTAMP(6) NULL
);

CREATE INDEX idx_outbox_events_unpublished ON outbox_events (created_at, published_at);
