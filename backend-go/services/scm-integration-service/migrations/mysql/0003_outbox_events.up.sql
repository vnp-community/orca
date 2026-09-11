-- MySQL/TiDB variant: payload JSON instead of JSONB — common/outbox.Relay
-- only reads payload whole to publish, never queries a JSON operator, so
-- no functional loss (mirrors usage-service's own outbox migration
-- comment verbatim, same package/relay on the read side).
CREATE TABLE outbox_events (
    id            CHAR(36) PRIMARY KEY,
    tenant_id     CHAR(36) NOT NULL,
    subject       VARCHAR(255) NOT NULL,
    occurred_at   TIMESTAMP(6) NOT NULL,
    version       INT NOT NULL,
    payload       JSON NOT NULL,
    created_at    TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    published_at  TIMESTAMP(6) NULL
);

-- MySQL has no partial index (Postgres's `WHERE published_at IS NULL`) —
-- indexes the full (created_at, published_at) pair instead, same
-- translation usage-service's own outbox migration already established.
CREATE INDEX idx_scm_outbox_events_unpublished ON outbox_events (created_at, published_at);

-- No Row-Level Security equivalent in MySQL/TiDB — same posture as
-- 0001_init.up.sql's tables:
--
--   ALTER TABLE scm.outbox_events ENABLE ROW LEVEL SECURITY;
--   CREATE POLICY tenant_isolation ON scm.outbox_events
--       USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
