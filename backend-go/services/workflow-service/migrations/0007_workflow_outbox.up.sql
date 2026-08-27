-- Transactional outbox table — same shape as usage.outbox_events
-- (usage-service/migrations/0002_outbox.up.sql). workflow.executions
-- writes and this table's INSERT happen in the same Postgres transaction
-- (internal/adapter/postgres.Repository.UpdateExecution); common/outbox.Relay
-- polls unpublished rows and publishes them to NATS JetStream.
CREATE TABLE workflow.outbox_events (
    id            UUID PRIMARY KEY,
    tenant_id     UUID NOT NULL,
    subject       TEXT NOT NULL,
    occurred_at   TIMESTAMPTZ NOT NULL,
    version       INT NOT NULL,
    payload       JSONB NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at  TIMESTAMPTZ
);

CREATE INDEX idx_workflow_outbox_events_unpublished
    ON workflow.outbox_events (created_at)
    WHERE published_at IS NULL;
