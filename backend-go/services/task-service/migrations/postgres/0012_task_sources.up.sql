-- Links a task to the external work item (Jira/Linear/GitHub/GitLab issue) it
-- was started from. A separate table, not columns on task.tasks: every Task
-- SELECT/INSERT list stays untouched, and a task without a source simply has
-- no row here.
--
-- The UNIQUE key makes "start work" idempotent: a second start on the same
-- issue within a project resolves to the existing task instead of creating a
-- duplicate (and a second worktree).
CREATE TABLE task.task_sources (
    task_id    UUID PRIMARY KEY REFERENCES task.tasks(id) ON DELETE CASCADE,
    tenant_id  UUID NOT NULL,
    project_id UUID,
    provider   TEXT NOT NULL CHECK (provider IN ('jira','linear','github','gitlab')),
    ref        TEXT NOT NULL,
    url        TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- COALESCE: a NULL project_id would otherwise never collide in a unique index.
CREATE UNIQUE INDEX idx_task_sources_unique
    ON task.task_sources (tenant_id, COALESCE(project_id, '00000000-0000-0000-0000-000000000000'::uuid), provider, ref);

ALTER TABLE task.task_sources ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON task.task_sources
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
