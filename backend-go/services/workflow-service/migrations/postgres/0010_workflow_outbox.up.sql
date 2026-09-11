-- Transactional outbox table — same shape as usage.outbox_events
-- (usage-service/migrations/0002_outbox.up.sql). Two independent event
-- families publish through this one table: workflow.executions writes
-- (internal/adapter/postgres.Repository.UpdateExecution, in the same
-- transaction) and BE-SOL-003/TASK-FT-003-03's step-level events — both
-- landed independently and were consolidated onto this single table
-- rather than creating a duplicate. common/outbox.Relay polls unpublished
-- rows and publishes them to NATS JetStream.
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
