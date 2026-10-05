-- MySQL translation of migrations/postgres/0014_task_sources_site.up.sql.
-- site_id is bounded (VARCHAR) because it is part of a unique key.
ALTER TABLE task_sources ADD COLUMN site_id VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE task_sources DROP INDEX idx_task_sources_unique;
ALTER TABLE task_sources ADD UNIQUE KEY idx_task_sources_unique (tenant_id, project_key, provider, site_id, ref);
