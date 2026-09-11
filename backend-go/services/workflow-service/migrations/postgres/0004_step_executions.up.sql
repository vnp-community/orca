-- Persists one row per step per wave within a WorkflowExecution's
-- wave-dispatch run — see workflow-service.md §4/§5 and domain.StepExecution.
-- Narrowed from the design doc's fuller schema (no started_at/completed_at
-- columns) to the columns this build's instructions name explicitly: id,
-- execution_id, step_id, wave, status, dispatch_token, output, error.
CREATE TABLE workflow.step_executions (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    execution_id   UUID NOT NULL REFERENCES workflow.executions(id) ON DELETE CASCADE,
    step_id        TEXT NOT NULL, -- references the execution's dag_json step id; steps aren't rows, not an FK
    wave           INT NOT NULL,
    status         TEXT NOT NULL DEFAULT 'pending'
                       CHECK (status IN ('pending', 'running', 'completed', 'failed')),
    dispatch_token UUID NOT NULL DEFAULT gen_random_uuid(), -- idempotency key for a future boot-time recovery scan, §8 (not implemented in this pass)
    output         JSONB,
    error_message  TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (execution_id, step_id)
);

CREATE INDEX idx_workflow_step_executions_execution ON workflow.step_executions (execution_id, wave);

-- RLS via execution_id join, per workflow-service.md §5 ("RLS on all three
-- tables (step_executions via execution_id join)") — this table carries no
-- tenant_id column of its own; the application layer scopes every query by
-- joining to workflow.executions (see internal/adapter/postgres), and this
-- policy is the secondary defense-in-depth backstop for that join.
ALTER TABLE workflow.step_executions ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON workflow.step_executions
    USING (EXISTS (
        SELECT 1 FROM workflow.executions e
        WHERE e.id = step_executions.execution_id
          AND e.tenant_id = current_setting('app.tenant_id', true)::uuid
    ));
