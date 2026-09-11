-- Transactional outbox table (see
-- specs/backend-go/architecture/05-data-architecture.md's "Transactional
-- outbox + async events (default)" section, and usage-service's
-- migrations/mysql/0002_outbox.up.sql for the reference shape).
-- automation_runs' terminal-status UPDATE and this table's INSERT happen
-- in the same database transaction (internal/adapter/mysql.
-- AutomationRunRepository.UpdateStatus writes this directly — see that
-- file's doc comment for why it doesn't reuse
-- internal/adapter/eventbus.RunCompletedPublisher, which is hard-coded to
-- pgx.Tx); common/outbox.Relay polls unpublished rows and publishes them
-- to NATS JetStream as orca.automation.run.completed. payload is JSON
-- (Postgres: JSONB, see 0003's comment for the JSONB->JSON rule).
CREATE TABLE outbox_events (
    id            CHAR(36) PRIMARY KEY,
    tenant_id     CHAR(36) NOT NULL,
    subject       TEXT NOT NULL,
    occurred_at   TIMESTAMP(6) NOT NULL,
    version       INT NOT NULL,
    payload       JSON NOT NULL,
    created_at    TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    published_at  TIMESTAMP(6) NULL
);

-- Postgres's idx_automation_outbox_events_unpublished is a PARTIAL index
-- (WHERE published_at IS NULL) — no MySQL equivalent, full index instead
-- (performance-only deviation, same category as 0002/0005's due/trigger
-- indexes above).
CREATE INDEX idx_automation_outbox_events_unpublished
    ON outbox_events (published_at, created_at);
