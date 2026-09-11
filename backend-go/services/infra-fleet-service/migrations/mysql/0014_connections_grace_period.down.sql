-- MySQL does not support "DROP COLUMN IF EXISTS" (confirmed against real
-- mysql:8 — a hard syntax error, unlike Postgres which accepts it).
ALTER TABLE connections
  DROP COLUMN degraded_since,
  DROP COLUMN grace_period_seconds;
