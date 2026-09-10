-- Transactional outbox table (BE-SOL-003/TASK-FT-003-01) — same shape as
-- usage.outbox_events (services/usage-service/migrations/0002_outbox.up.sql),
-- the working precedent for common/outbox.Relay in this codebase.
-- Numbered 0005, not the CR's own "0004" guess — migrations 0001-0004
-- already exist at implementation time (0004_coordinator_run_lifecycle,
-- TASK-TASKV1-005), so this is the genuinely next-free number.
CREATE TABLE orchestration.outbox_events (
    id            UUID PRIMARY KEY,
    tenant_id     UUID NOT NULL,
    subject       TEXT NOT NULL,
    occurred_at   TIMESTAMPTZ NOT NULL,
    version       INT NOT NULL,
    payload       JSONB NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at  TIMESTAMPTZ
);

-- Partial index over only the rows the relay actually polls — stays small
-- and fast regardless of how large the fully-published history grows.
CREATE INDEX idx_orchestration_outbox_events_unpublished
    ON orchestration.outbox_events (created_at)
    WHERE published_at IS NULL;
