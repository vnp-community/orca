-- issue-tracking-service's first (and, deliberately, only) database table —
-- added purely to host the transactional-outbox row for Epic G
-- (docs/execution-plan.md). This is NOT a queryable copy of issue data:
-- Jira/Linear remain the systems of record (design doc §2/§5). See
-- internal/adapter/postgres's package doc comment for the full reasoning.
CREATE SCHEMA IF NOT EXISTS issuetracking;

CREATE TABLE issuetracking.outbox_events (
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
CREATE INDEX idx_issuetracking_outbox_events_unpublished
    ON issuetracking.outbox_events (created_at)
    WHERE published_at IS NULL;

-- Row-Level Security as defense-in-depth per architecture/05 — the
-- application layer's explicit tenant_id filtering (internal/adapter/postgres)
-- is the primary enforcement; this is the secondary backstop, matching
-- every other service's outbox/domain tables in this scaffold.
ALTER TABLE issuetracking.outbox_events ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON issuetracking.outbox_events
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
