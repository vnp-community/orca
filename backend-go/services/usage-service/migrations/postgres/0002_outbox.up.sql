-- Transactional outbox table (Epic G, docs/execution-plan.md) — see
-- specs/backend-go/architecture/05-data-architecture.md's "Transactional
-- outbox + async events (default)" section. usage.sessions writes and this
-- table's INSERT happen in the same Postgres transaction
-- (internal/adapter/postgres.Repository.SaveSession); common/outbox.Relay
-- polls unpublished rows and publishes them to NATS JetStream.
CREATE TABLE usage.outbox_events (
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
CREATE INDEX idx_usage_outbox_events_unpublished
    ON usage.outbox_events (created_at)
    WHERE published_at IS NULL;
