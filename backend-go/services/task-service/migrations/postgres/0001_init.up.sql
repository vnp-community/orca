-- task-service owns this database exclusively — no other service reads or
-- writes these tables. See specs/backend-go/architecture/05-data-architecture.md.
--
-- Bounded context note (task-service.md §2): this schema does NOT own
-- team_members — that table belongs to tenant-service. task_grants.subject_id
-- may reference a team ID (level='team') or a company/tenant ID
-- (level='company') in addition to a user ID (level in owner/admin/user),
-- but resolving "is this user a member of that team" happens via a gRPC
-- call to tenant-service (see internal/usecase.TeamScopeResolver), never a
-- local join against a team_members table this service doesn't have.
CREATE SCHEMA IF NOT EXISTS task;

CREATE TABLE task.tasks (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL,
    title       TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'in_progress', 'done', 'cancelled')),
    -- parent_id is denormalized directly onto the task row so
    -- GetAncestors (usecase.TaskRepository) can walk the hierarchy with one
    -- WITH RECURSIVE query without requiring a task_edges parent_child row
    -- to exist first — see task-service.md §6.
    parent_id   UUID REFERENCES task.tasks(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_tasks_tenant ON task.tasks (tenant_id);
CREATE INDEX idx_tasks_parent ON task.tasks (parent_id);

-- Row-Level Security as defense-in-depth per architecture/05 — the
-- application layer's explicit tenant_id filtering (see internal/adapter/postgres)
-- is the primary enforcement; this is the secondary backstop.
ALTER TABLE task.tasks ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON task.tasks
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- edge_type: 'parent_child' (hierarchy — kept here too, redundantly with
-- tasks.parent_id, so AddEdge/RemoveEdge have one representation to mutate
-- for future re-parenting support) or 'depends_on' (ordering, walked by
-- GetDependencies + the complex-Execute path and protected by
-- domain.DetectCycle, this service's cycle detector).
CREATE TABLE task.task_edges (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    UUID NOT NULL,
    from_task_id UUID NOT NULL REFERENCES task.tasks(id) ON DELETE CASCADE,
    to_task_id   UUID NOT NULL REFERENCES task.tasks(id) ON DELETE CASCADE,
    edge_type    TEXT NOT NULL CHECK (edge_type IN ('parent_child', 'depends_on')),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (from_task_id, to_task_id, edge_type)
);
-- one parent_child edge per child; both directions indexed for
-- ancestor/descendant/dependency traversal.
CREATE UNIQUE INDEX task_edges_single_parent ON task.task_edges (to_task_id) WHERE edge_type = 'parent_child';
CREATE INDEX task_edges_from_idx ON task.task_edges (tenant_id, from_task_id, edge_type);
CREATE INDEX task_edges_to_idx   ON task.task_edges (tenant_id, to_task_id, edge_type);

ALTER TABLE task.task_edges ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON task.task_edges
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- level folds grantee-kind into the permission tier itself, matching the
-- generated proto's GrantLevel enum (owner/admin/user are direct per-user
-- grants; team/company are scope-wide grants whose subject_id names the
-- team/company) — see internal/domain/grant.go's doc comment for why this
-- scaffold follows the proto rather than the design doc's separate
-- grantee_type sketch.
CREATE TABLE task.task_grants (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id  UUID NOT NULL,
    task_id    UUID NOT NULL REFERENCES task.tasks(id) ON DELETE CASCADE,
    subject_id UUID NOT NULL,
    level      TEXT NOT NULL CHECK (level IN ('owner', 'admin', 'user', 'team', 'company')),
    apply_tree BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX task_grants_task_idx ON task.task_grants (tenant_id, task_id);

ALTER TABLE task.task_grants ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON task.task_grants
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);

CREATE TABLE task.task_comments (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id  UUID NOT NULL,
    task_id    UUID NOT NULL REFERENCES task.tasks(id) ON DELETE CASCADE,
    author_id  UUID NOT NULL,             -- logical FK -> tenant-service
    content    TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX task_comments_task_idx ON task.task_comments (tenant_id, task_id);

ALTER TABLE task.task_comments ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON task.task_comments
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
