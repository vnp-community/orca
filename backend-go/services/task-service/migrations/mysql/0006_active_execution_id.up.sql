-- MySQL translation of migrations/postgres/0006_active_execution_id.up.sql.
ALTER TABLE tasks ADD COLUMN active_execution_id TEXT;
