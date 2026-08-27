-- Transactional outbox table (see
-- specs/backend-go/architecture/05-data-architecture.md's "Transactional
-- outbox + async events (default)" section, and usage-service's
-- 0002_outbox.up.sql for the reference shape). automation.automation_runs'
-- terminal-status UPDATE and this table's INSERT happen in the same
-- Postgres transaction (internal/adapter/postgres.AutomationRunRepository.
-- UpdateStatus, via internal/adapter/eventbus.RunCompletedPublisher);
-- common/outbox.Relay polls unpublished rows and publishes them to NATS
-- JetStream as orca.automation.run.completed.
CREATE TABLE automation.outbox_events (
    id            UUID PRIMARY KEY,
    tenant_id     UUID NOT NULL,
    subject       TEXT NOT NULL,
    occurred_at   TIMESTAMPTZ NOT NULL,
    version       INT NOT NULL,
    payload       JSONB NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at  TIMESTAMPTZ
);

CREATE INDEX idx_automation_outbox_events_unpublished
    ON automation.outbox_events (created_at)
    WHERE published_at IS NULL;
