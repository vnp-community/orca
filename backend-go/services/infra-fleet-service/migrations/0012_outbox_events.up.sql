-- Transactional outbox table (docs/execution-plan.md Epic G; see
-- specs/backend-go/architecture/05-data-architecture.md's "Transactional
-- outbox + async events (default)" section) — mirrors usage.outbox_events'
-- schema exactly (common/outbox.Store's contract). First real use: alerting
-- admins when a dev server's fleet-health sample transitions
-- reachable=true -> false (see usecase.PollFleetHealth).
--
-- IF NOT EXISTS: a stray, schema-identical table with this exact shape was
-- found live on b15's infra database during this feature's investigation,
-- created out-of-band by an unrelated exploration pass — this migration is
-- a no-op there and a real create everywhere else.
CREATE TABLE IF NOT EXISTS infra.outbox_events (
    id            UUID PRIMARY KEY,
    tenant_id     UUID NOT NULL,
    subject       TEXT NOT NULL,
    occurred_at   TIMESTAMPTZ NOT NULL,
    version       INT NOT NULL,
    payload       JSONB NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at  TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_infra_outbox_events_unpublished
    ON infra.outbox_events (created_at)
    WHERE published_at IS NULL;
