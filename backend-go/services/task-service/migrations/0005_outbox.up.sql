-- Transactional-outbox table for task-service's grant-mutation audit
-- events (task.grant_received / task.grant_revoked) — see
-- common/outbox.Relay's doc comment and usage-service's identical
-- usage.outbox_events table for the pattern this mirrors.
CREATE TABLE task.outbox_events (
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
CREATE INDEX idx_task_outbox_events_unpublished
    ON task.outbox_events (created_at)
    WHERE published_at IS NULL;
