-- issue-status-sync owns this database exclusively — no other service
-- reads or writes this table. MySQL/TiDB variant: no CREATE SCHEMA (a
-- MySQL database IS the schema-equivalent isolation unit — this migration
-- assumes DATABASE_DSN already points at a database named
-- `issuestatussync`, mirroring the Postgres variant's `issuestatussync`
-- schema name; see
-- specs/backend-go/crs/v4/multi-database/solutions/BE-DB-SOL-003.md).
--
-- event_id is TEXT in the Postgres variant (unbounded); MySQL/InnoDB
-- rejects TEXT/BLOB as a PRIMARY KEY without an explicit key-prefix
-- length, so this uses VARCHAR(255) instead — wide enough for the UUID-
-- shaped IDs common/eventbus.Event.ID actually carries today, consistent
-- with the VARCHAR(255) bound already used for usage-service's own
-- opaque-string columns (see usage-service/migrations/mysql/0002_outbox.up.sql's
-- `subject` column).
CREATE TABLE processed_events (
    event_id     VARCHAR(255) PRIMARY KEY,
    processed_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
);
-- Dedup cache only (08-inter-service-communication.md:42-45), not an audit
-- log — a short-TTL cleanup job (e.g. 7-day retention) should be added as a
-- follow-up, not required for this task.
--
-- No Row-Level Security equivalent needed here at all — this table has no
-- tenant_id column in either dialect (event_id/processed_at only), so
-- there is no per-tenant data to isolate and no compensating-control test
-- is applicable (see TASK-BE-DB-008's "Kết quả thực tế" for the explicit
-- note this was checked, not assumed).
