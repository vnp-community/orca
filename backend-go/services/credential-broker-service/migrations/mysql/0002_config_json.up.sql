-- config_json is a non-secret sidecar config column (TASK-037/038) — NEVER
-- a secret value, same "no secret columns, ever" discipline 0001_init.up.sql
-- documents for the rest of this table. TEXT in both dialects — this column
-- was never JSONB in the Postgres original, so there is no JSON-operator
-- loss here (unlike usage-service's outbox payload column).
ALTER TABLE credential_metadata ADD COLUMN config_json TEXT;
