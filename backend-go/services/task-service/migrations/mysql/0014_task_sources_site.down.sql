-- Lossy: rows differing only by site collapse into one key, so duplicates are
-- dropped (keeping the oldest) before the narrower unique index is restored.
ALTER TABLE task_sources DROP INDEX idx_task_sources_unique;
DELETE a FROM task_sources a JOIN task_sources b
  ON a.tenant_id = b.tenant_id AND a.project_key = b.project_key
 AND a.provider = b.provider AND a.ref = b.ref
 AND (a.created_at > b.created_at OR (a.created_at = b.created_at AND a.task_id > b.task_id));
ALTER TABLE task_sources ADD UNIQUE KEY idx_task_sources_unique (tenant_id, project_key, provider, ref);
ALTER TABLE task_sources DROP COLUMN site_id;
