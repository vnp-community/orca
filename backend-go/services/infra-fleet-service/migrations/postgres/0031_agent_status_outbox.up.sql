-- rate-limited events only — statusChanged never touches the DB, see
-- adapter/eventbus/agent_status_publisher.go's package doc comment for why
-- (TASK-AG-05-05: a deliberate, signed-off exception to
-- 08-inter-service-communication.md's outbox-always rule).
CREATE TABLE infra.agent_rate_limited_outbox_events (
    id           UUID PRIMARY KEY,
    tenant_id    UUID NOT NULL,
    subject      TEXT NOT NULL,
    occurred_at  TIMESTAMPTZ NOT NULL,
    version      INT NOT NULL DEFAULT 1,
    payload      JSONB NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ
);
CREATE INDEX idx_infra_agent_rate_limited_outbox_unpublished
    ON infra.agent_rate_limited_outbox_events (created_at)
    WHERE published_at IS NULL;
