-- project.worktrees — worktree metadata, explicitly NOT authoritative for
-- on-disk existence (see domain.Worktree's doc comment). ON DELETE CASCADE
-- on both project_id and repo_id: a worktree row has no independent meaning
-- once its owning project or repo is gone.
CREATE TABLE project.worktrees (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES project.projects (id) ON DELETE CASCADE,
    repo_id    UUID NOT NULL REFERENCES project.repos (id) ON DELETE CASCADE,
    path       TEXT NOT NULL,
    branch     TEXT NOT NULL,
    active     BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_worktrees_project ON project.worktrees (project_id);
CREATE INDEX idx_worktrees_repo ON project.worktrees (repo_id);

ALTER TABLE project.worktrees ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON project.worktrees
    USING (project_id IN (
        SELECT id FROM project.projects
        WHERE tenant_id = current_setting('app.tenant_id', true)::uuid
    ));
