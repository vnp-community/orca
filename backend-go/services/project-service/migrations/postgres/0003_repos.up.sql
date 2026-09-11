-- project.repos — a project's repo catalog. Metadata only (see
-- project-service.md §4's Repo entity); this is the slice of that fuller
-- model (path/remote_url/icon_ref) the current proto surface (AddRepo/
-- ListRepos/ReorderRepos/RemoveRepo) actually exercises.
--
-- ON DELETE CASCADE on project_id: a repo row has no independent meaning
-- once its owning project is gone — see usecase.DeleteProject's doc comment.
CREATE TABLE project.repos (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id   UUID NOT NULL REFERENCES project.projects (id) ON DELETE CASCADE,
    url          TEXT NOT NULL,
    display_name TEXT NOT NULL DEFAULT '',
    position     INT  NOT NULL DEFAULT 0,   -- ordering within project; not
                                             -- unique — see domain.Repo.Position
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_repos_project ON project.repos (project_id);

ALTER TABLE project.repos ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON project.repos
    USING (project_id IN (
        SELECT id FROM project.projects
        WHERE tenant_id = current_setting('app.tenant_id', true)::uuid
    ));
