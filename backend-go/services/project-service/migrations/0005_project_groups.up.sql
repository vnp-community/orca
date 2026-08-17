-- project.project_groups — self-referential folder-style organization tree,
-- tenant-scoped (not tied to a specific project — see domain.ProjectGroup's
-- doc comment on this being a slice of project-service.md §4's fuller
-- model). ON DELETE CASCADE on parent_group_id: deleting a group deletes
-- its descendant subtree — see usecase.DeleteProjectGroup's doc comment.
CREATE TABLE project.project_groups (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL,
    name            TEXT NOT NULL,
    parent_group_id UUID REFERENCES project.project_groups (id) ON DELETE CASCADE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_project_groups_tenant ON project.project_groups (tenant_id);
CREATE INDEX idx_project_groups_parent ON project.project_groups (parent_group_id);

ALTER TABLE project.project_groups ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON project.project_groups
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
