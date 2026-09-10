-- Transactional outbox table (Epic G, docs/execution-plan.md; TASK-AUTH-05-08)
-- — see specs/backend-go/architecture/05-data-architecture.md's
-- "Transactional outbox + async events (default)" section. infra.connections
-- writes and this table's INSERT happen in the same Postgres transaction
-- (internal/adapter/postgres.Repository.CreateConnectionWithOutbox);
-- common/outbox.Relay polls unpublished rows and publishes them to NATS
-- JetStream. Mirrors usage-service's usage.outbox_events table exactly.
CREATE TABLE infra.outbox_events (
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
CREATE INDEX idx_infra_outbox_events_unpublished
    ON infra.outbox_events (created_at)
    WHERE published_at IS NULL;
