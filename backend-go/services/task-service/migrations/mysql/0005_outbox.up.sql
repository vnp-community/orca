-- MySQL translation of migrations/postgres/0005_outbox.up.sql.
CREATE TABLE outbox_events (
    id            CHAR(36) NOT NULL PRIMARY KEY, -- always caller-supplied (event.ID), no DEFAULT needed
    tenant_id     CHAR(36) NOT NULL,
    subject       TEXT NOT NULL,
    occurred_at   TIMESTAMP(6) NOT NULL,
    version       INT NOT NULL,
    payload       JSON NOT NULL,
    created_at    TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    published_at  TIMESTAMP(6) NULL
) ENGINE=InnoDB;

-- Postgres's idx_task_outbox_events_unpublished is a PARTIAL index
-- (WHERE published_at IS NULL) — MySQL has no partial index; a composite
-- index leading with published_at instead keeps the relay's `WHERE
-- published_at IS NULL ORDER BY created_at` query index-only for the
-- unpublished rows it actually polls, same approach usage-service's MySQL
-- migration uses for its own outbox table (BE-DB-SOL-001 §5).
CREATE INDEX idx_task_outbox_events_unpublished ON outbox_events (published_at, created_at);
