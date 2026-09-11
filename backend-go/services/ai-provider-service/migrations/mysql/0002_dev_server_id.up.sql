-- Mirrors the Postgres variant's 0002_dev_server_id.up.sql exactly (same
-- columns, same rationale — dev_server_id/soft-delete marker/label/
-- model_hint/base_url). dev_server_id/label/base_url/model_hint are
-- VARCHAR here rather than Postgres's unbounded TEXT: MySQL 8.0.13+ does
-- allow a literal DEFAULT on TEXT/BLOB only via an expression
-- (`DEFAULT ('')`), not the plain `DEFAULT ''` this column needs, so a
-- bounded VARCHAR is used instead — same choice credential-broker-service's
-- MySQL migration made for its own label/pointer columns (BE-DB-SOL-006).
-- Every value stored here (a dev-server id, a short user-facing label, a
-- model-hint string, a base URL) comfortably fits the caps chosen; base_url
-- gets a wider cap since URLs can run long.
ALTER TABLE accounts
  ADD COLUMN dev_server_id VARCHAR(255) NOT NULL DEFAULT '',
  ADD COLUMN deleted_at TIMESTAMP(6) NULL,
  ADD COLUMN label VARCHAR(255) NOT NULL DEFAULT '',
  ADD COLUMN model_hint VARCHAR(255) NULL,
  ADD COLUMN base_url VARCHAR(2048) NULL;
