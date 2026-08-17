-- workflow-service owns this database exclusively — no other service reads
-- or writes these tables. See specs/backend-go/architecture/05-data-architecture.md.
CREATE SCHEMA IF NOT EXISTS workflow;

-- Reusable, named DAG definitions — see workflow-service.md §4/§5. Narrowed
-- from the design doc's fuller schema (version/parent_template_id/owner_id/
-- description columns) to the columns this scaffold's build instructions
-- name explicitly: id, tenant_id, name, dag_json, scope. Extend before
-- template inheritance resolution (§4/§6) is implemented.
CREATE TABLE workflow.templates (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL,
    name        TEXT NOT NULL,
    dag_json    JSONB NOT NULL DEFAULT '{"steps":[]}',
    scope       TEXT NOT NULL DEFAULT 'personal' CHECK (scope IN ('company', 'team', 'personal')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_workflow_templates_tenant ON workflow.templates (tenant_id, name);

-- Row-Level Security as defense-in-depth per architecture/05 — the
-- application layer's explicit tenant_id filtering (see internal/adapter/postgres)
-- is the primary enforcement; this is the secondary backstop.
ALTER TABLE workflow.templates ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON workflow.templates
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- Runs of a template's DAG — see workflow-service.md §4/§5. root_trace_id
-- and paused_at are carried forward unconditionally per the design doc's
-- resumability (§8) and user-triggered pause (§4-5) hard requirements —
-- the Go equivalent of TS migrations 0013_workflow_trace_correlation and
-- 0014_workflow_pause_state.
CREATE TABLE workflow.executions (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    template_id    UUID NOT NULL REFERENCES workflow.templates(id) ON DELETE CASCADE,
    tenant_id      UUID NOT NULL,
    status         TEXT NOT NULL DEFAULT 'pending'
                       CHECK (status IN ('pending', 'running', 'paused', 'completed', 'failed', 'cancelled')),
    root_trace_id  TEXT,          -- restart correlation, §8 hard requirement
    paused_at      TIMESTAMPTZ,   -- user-triggered pause; NULL = not paused, cleared on resume, §4-5
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_workflow_executions_tenant_status ON workflow.executions (tenant_id, status, created_at DESC);
-- Boot-time recovery scan target, §8: every running/paused execution on
-- startup, before accepting new Execute calls (recovery scan itself is not
-- implemented in this scaffold — see README "Known gaps" — but the index
-- it would use is here from day one).
CREATE INDEX idx_workflow_executions_resumable ON workflow.executions (status) WHERE status IN ('running', 'paused');

ALTER TABLE workflow.executions ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON workflow.executions
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
