-- MySQL/TiDB variant of postgres/0005_outbox_events.up.sql — same shape as
-- usage-service's/issue-tracking-service's own outbox_events MySQL table,
-- the working precedent for common/outbox.Relay in this codebase. payload
-- JSON instead of JSONB (MySQL 5.7+/TiDB has native JSON but no JSONB
-- ->/@> operators) — common/outbox.Relay only reads payload whole to
-- publish it, never queries by JSON operator, so nothing is lost here.
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

-- Plain composite index — Postgres's equivalent is a partial index
-- (WHERE published_at IS NULL); see 0001_init's comment on this trade-off.
CREATE INDEX idx_orchestration_outbox_events_unpublished ON outbox_events (created_at, published_at);
