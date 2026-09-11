-- Transactional-outbox table for task-service's published events — see
-- common/outbox.Relay's doc comment and usage-service's identical
-- usage.outbox_events table for the pattern this mirrors. Two independent
-- event families publish through this one table: grant-mutation audit
-- events (task.grant_received / task.grant_revoked, TG-03-07) and
-- TASK-AG-FLOWTASK-003's orca.task.agent_output_partial (Engine 1's
-- throttled mid-run output, republished onto task.activity:{taskId} by
-- api-gateway's wscompat layer) — both landed independently and were
-- consolidated onto this single table rather than creating a duplicate.
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
