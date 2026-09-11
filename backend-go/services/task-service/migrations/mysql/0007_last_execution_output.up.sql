-- MySQL translation of migrations/postgres/0007_last_execution_output.up.sql.
-- Bounded to 8KB via application-layer truncation (internal/adapter/mysql's
-- UpdateLastExecutionOutput, mirroring the Postgres adapter) — TEXT's 64KB
-- capacity comfortably covers that bound.
ALTER TABLE tasks ADD COLUMN last_execution_output TEXT;
