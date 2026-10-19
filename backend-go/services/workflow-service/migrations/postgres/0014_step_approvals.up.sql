-- Decisions a running workflow step waits for from a person in Orca (TDD-WF-AGENT-001 §7.5), e.g. a tdd-agent gate's
-- approval. One row per (step, decision shown): a newer decision for the same step supersedes the pending one.
CREATE TABLE workflow.step_approvals (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     UUID NOT NULL,
    execution_id  UUID NOT NULL REFERENCES workflow.executions(id) ON DELETE CASCADE,
    step_id       TEXT NOT NULL,
    project_id    TEXT NOT NULL DEFAULT '',
    kind          TEXT NOT NULL,
    subject       JSONB NOT NULL DEFAULT '{}',
    decision      JSONB,
    decision_hash TEXT NOT NULL,
    status        TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected', 'superseded')),
    resolved_by   TEXT,
    rationale     TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at   TIMESTAMPTZ,
    submitted_at  TIMESTAMPTZ,
    UNIQUE (execution_id, step_id, decision_hash)
);

CREATE INDEX idx_workflow_step_approvals_pending ON workflow.step_approvals (tenant_id, project_id) WHERE status = 'pending';
CREATE INDEX idx_workflow_step_approvals_step ON workflow.step_approvals (execution_id, step_id);

ALTER TABLE workflow.step_approvals ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON workflow.step_approvals
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
