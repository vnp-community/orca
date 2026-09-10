-- infra-fleet-service had no transactional-outbox table before this —
-- infra-fleet-service.md §7 already documents dev_server.health_degraded
-- as an intended NATS JetStream event published via the outbox pattern,
-- but no writer or outbox infrastructure existed yet (TASK-FLEET-03-06).
-- Mirrors usage.outbox_events's shape (usage-service/migrations/0002_outbox).
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
