-- project-service owns this database exclusively — no other service reads
-- or writes these tables. See specs/backend-go/architecture/05-data-architecture.md.
--
-- This is the slice of project-service.md §5's full schema that the current
-- proto surface (CreateProject/GetProject/ListProjects/AddMember/
-- RebindDevServer) exercises: projects + project_members only. The richer
-- schema (repos, worktrees, project_groups, source_projects) is a documented
-- follow-up, not created here — see this service's README "Known gaps".
CREATE SCHEMA IF NOT EXISTS project;

CREATE TABLE project.projects (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL,
    name            TEXT NOT NULL,
    dev_server_id   UUID,          -- logical FK -> infra-fleet-service.dev_servers; nullable
                                    -- until the first RebindDevServer call (see internal/domain/project.go)
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_projects_tenant ON project.projects (tenant_id);
CREATE INDEX idx_projects_dev_server ON project.projects (dev_server_id);

-- Row-Level Security as defense-in-depth per architecture/05 — the
-- application layer's explicit tenant_id filtering (see internal/adapter/postgres)
-- is the primary enforcement; this is the secondary backstop.
ALTER TABLE project.projects ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON project.projects
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);

CREATE TABLE project.project_members (
    project_id  UUID NOT NULL REFERENCES project.projects (id) ON DELETE CASCADE,
    user_id     UUID NOT NULL,     -- logical FK -> tenant-service.users
    role        TEXT NOT NULL CHECK (role IN ('member', 'owner')),
    added_at    TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (project_id, user_id)
);
CREATE INDEX idx_project_members_user ON project.project_members (user_id);

-- project_members has no tenant_id column of its own (see
-- project-service.md §5's schema) — tenant scoping is transitive through the
-- projects FK, so the RLS policy is a subquery against project.projects
-- rather than a denormalized tenant_id copy.
ALTER TABLE project.project_members ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON project.project_members
    USING (project_id IN (
        SELECT id FROM project.projects
        WHERE tenant_id = current_setting('app.tenant_id', true)::uuid
    ));
