-- MySQL/TiDB variant of postgres/0001_outbox.up.sql. No CREATE SCHEMA — a
-- MySQL database IS the schema-equivalent isolation unit; DATABASE_DSN for
-- this dialect points at a database already named `issuetracking`, mirroring
-- the Postgres variant's `issuetracking` schema (see
-- specs/backend-go/crs/v4/multi-database/solutions/BE-DB-SOL-002.md, the
-- naming decision this service's rollout reuses unchanged).
--
-- payload JSON instead of JSONB (MySQL 5.7+/TiDB has native JSON but no
-- JSONB ->/@> operators) — common/outbox.Relay only reads payload whole to
-- publish it, never queries by JSON operator, so nothing is lost here.
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

-- MySQL has no partial/filtered index — a plain composite index over
-- (created_at, published_at) serves the same "oldest unpublished first"
-- poll the postgres variant's partial index was for, just without the
-- "small regardless of published history size" property.
CREATE INDEX idx_outbox_events_unpublished ON outbox_events (created_at, published_at);

-- No Row-Level Security equivalent in MySQL/TiDB — application-layer
-- tenant_id scoping in internal/adapter/mysql is the ONLY enforcement here,
-- not a secondary backstop. Per BE-DB-SOL-001 §4's finding (confirmed by
-- grep: no `SET LOCAL app.tenant_id` call exists anywhere in backend-go),
-- the Postgres RLS policy this table's pilot equivalent carried never
-- actually activated either — this migration doesn't regress protection,
-- it stops declaring a backstop that was never real.
