-- source_projects: links source_project_id's repos/worktrees into
-- container_project_id's shared view. Both sides are ordinary
-- project.projects rows — there is no separate "OrcaProject" entity in
-- this service (see project.proto's orcaProjects.* section doc comment).
-- linked_by is an audit trail (who performed the link), not an ownership
-- claim on either project — access is still gated entirely through
-- container_project_id's own project_members/OPA check (requireProjectAccess),
-- enforced in Go, not here.
CREATE TABLE project.source_projects (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    container_project_id  UUID NOT NULL REFERENCES project.projects(id) ON DELETE CASCADE,
    source_project_id     UUID NOT NULL REFERENCES project.projects(id) ON DELETE CASCADE,
    linked_by             UUID NOT NULL,
    linked_at             TIMESTAMPTZ NOT NULL DEFAULT now(),

    CHECK (container_project_id != source_project_id),
    UNIQUE (container_project_id, source_project_id)
);
CREATE INDEX idx_source_projects_container ON project.source_projects (container_project_id);

-- No tenant_id column of its own — same convention as repo_members
-- (0011's comment): tenant scoping is transitive through
-- container_project_id -> project.projects.tenant_id.
ALTER TABLE project.source_projects ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON project.source_projects
    USING (container_project_id IN (
        SELECT p.id FROM project.projects p
        WHERE p.tenant_id = current_setting('app.tenant_id', true)::uuid
    ));
