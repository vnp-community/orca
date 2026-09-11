-- config_json is a non-secret sidecar config column (TASK-037/038) — NEVER
-- a secret value, same "no secret columns, ever" discipline 0001_init.up.sql
-- documents for the rest of this table.
ALTER TABLE credential.credential_metadata ADD COLUMN config_json TEXT;
