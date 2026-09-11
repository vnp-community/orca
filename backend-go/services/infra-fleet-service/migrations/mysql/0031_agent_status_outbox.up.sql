-- rate-limited events only — statusChanged never touches the DB, see
-- adapter/eventbus/agent_status_publisher.go's package doc comment for why
-- (TASK-AG-05-05: a deliberate, signed-off exception to
-- 08-inter-service-communication.md's outbox-always rule).
CREATE TABLE agent_rate_limited_outbox_events (
    id           CHAR(36) PRIMARY KEY,
    tenant_id    CHAR(36) NOT NULL,
    subject      TEXT NOT NULL,
    occurred_at  TIMESTAMP(6) NOT NULL,
    version      INT NOT NULL DEFAULT 1,
    payload      JSON NOT NULL,
    created_at   TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    published_at TIMESTAMP(6)
);
-- MySQL/TiDB has no partial index — see migrations/mysql/0025_outbox.up.sql's
-- same note.
CREATE INDEX idx_infra_agent_rate_limited_outbox_unpublished
    ON agent_rate_limited_outbox_events (created_at);
