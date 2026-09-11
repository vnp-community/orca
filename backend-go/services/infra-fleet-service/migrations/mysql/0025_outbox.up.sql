-- Transactional outbox table (Epic G, docs/execution-plan.md; TASK-AUTH-05-08)
-- — see specs/backend-go/architecture/05-data-architecture.md's
-- "Transactional outbox + async events (default)" section. connections
-- writes and this table's INSERT happen in the same Postgres transaction
-- (internal/adapter/postgres.Repository.CreateConnectionWithOutbox);
-- common/outbox.Relay polls unpublished rows and publishes them to NATS
-- JetStream. Mirrors usage-service's usage.outbox_events table exactly.
CREATE TABLE outbox_events (
    id            CHAR(36) PRIMARY KEY,
    tenant_id     CHAR(36) NOT NULL,
    subject       TEXT NOT NULL,
    occurred_at   TIMESTAMP(6) NOT NULL,
    version       INT NOT NULL,
    payload       JSON NOT NULL,
    created_at    TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    published_at  TIMESTAMP(6)
);

-- MySQL/TiDB has no partial index — full index instead of Postgres's
-- `WHERE published_at IS NULL`. Same table-size argument as usage-service's
-- own outbox translation: mostly-published history grows this index,
-- unlike the Postgres partial one, but relay poll correctness is unchanged.
CREATE INDEX idx_infra_outbox_events_unpublished
    ON outbox_events (created_at);
