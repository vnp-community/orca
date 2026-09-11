ALTER TABLE ssh_targets
  -- MySQL 8.0.13+ requires TEXT column defaults to be a parenthesized
  -- expression — see migrations/mysql/0002_connections.up.sql's comment.
  ADD COLUMN project TEXT NOT NULL DEFAULT (''),
  -- Postgres's TEXT[] has no MySQL/TiDB equivalent array type — stored as a
  -- JSON array of strings instead (internal/adapter/mysql marshals/
  -- unmarshals []string on every read/write, no JSON operator needed —
  -- same scope as ai-provider-service's migrations/mysql/
  -- 0003_account_registration_fields.up.sql precedent). NOT NULL DEFAULT
  -- (JSON_ARRAY()) mirrors Postgres's NOT NULL DEFAULT '{}' (empty array,
  -- never NULL); MySQL requires JSON/TEXT/BLOB defaults to be a
  -- parenthesized expression, not a plain literal.
  ADD COLUMN tags     JSON NOT NULL DEFAULT (JSON_ARRAY());

-- Upsert-by-hostname+user (BL-FLEET-01's "INSERT OR UPDATE by hostname+user")
-- needs this uniqueness constraint to exist at all — it does not today.
-- host/user_name are TEXT (unbounded) — MySQL/InnoDB needs an explicit
-- prefix length for any indexed TEXT column (see migrations/mysql/
-- 0017_ssh_targets_host_unique.up.sql's same fix for `host` alone).
CREATE UNIQUE INDEX idx_infra_ssh_targets_tenant_host_user
  ON ssh_targets (tenant_id, host(255), user_name(255));
