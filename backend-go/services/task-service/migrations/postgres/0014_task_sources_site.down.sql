-- Lossy: rows differing only by site collapse into one key, so duplicates are
-- dropped (keeping the oldest) before the narrower unique index is restored.
DROP INDEX IF EXISTS task.idx_task_sources_unique;
DELETE FROM task.task_sources a USING task.task_sources b
 WHERE a.tenant_id = b.tenant_id
   AND COALESCE(a.project_id, '00000000-0000-0000-0000-000000000000'::uuid) = COALESCE(b.project_id, '00000000-0000-0000-0000-000000000000'::uuid)
   AND a.provider = b.provider AND a.ref = b.ref
   AND (a.created_at, a.task_id) > (b.created_at, b.task_id);
CREATE UNIQUE INDEX idx_task_sources_unique
    ON task.task_sources (tenant_id, COALESCE(project_id, '00000000-0000-0000-0000-000000000000'::uuid), provider, ref);
ALTER TABLE task.task_sources DROP COLUMN site_id;
