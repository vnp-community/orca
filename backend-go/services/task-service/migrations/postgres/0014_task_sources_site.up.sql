-- A Jira key (ENG-1) is only unique within one Jira site, so the source link
-- records the connection workspace id (site base URL). '' = unknown (rows that
-- predate this migration); lookups treat it as a wildcard for any site.
ALTER TABLE task.task_sources ADD COLUMN site_id TEXT NOT NULL DEFAULT '';

DROP INDEX task.idx_task_sources_unique;
CREATE UNIQUE INDEX idx_task_sources_unique
    ON task.task_sources (tenant_id, COALESCE(project_id, '00000000-0000-0000-0000-000000000000'::uuid), provider, site_id, ref);
