CREATE TABLE outbox_events (
    id            CHAR(36) PRIMARY KEY,
    tenant_id     CHAR(36) NOT NULL,
    subject       TEXT NOT NULL,
    occurred_at   DATETIME(6) NOT NULL,
    version       INT NOT NULL,
    payload       JSON NOT NULL,
    created_at    DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    published_at  DATETIME(6)
);

-- Postgres source is a PARTIAL index (`WHERE published_at IS NULL`) — no
-- MySQL equivalent, so this is a full composite index instead
-- (annotation-service's BE-DB-SOL-005 §5 precedent). (published_at,
-- created_at) still serves FetchUnpublished's
-- `WHERE published_at IS NULL ORDER BY created_at` efficiently.
CREATE INDEX idx_project_outbox_events_unpublished ON outbox_events (published_at, created_at);

-- RLS dropped — see 0001_init.up.sql's comment.
