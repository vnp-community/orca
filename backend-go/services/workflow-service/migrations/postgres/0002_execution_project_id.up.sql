-- Adds project_id to workflow.executions so HasActiveExecutions (Epic C,
-- backend-go/docs/execution-plan.md) can answer "does this project have a
-- non-terminal execution" without scanning every tenant's executions.
-- project-service.RebindDevServer's active-execution guard was previously a
-- no-op because neither this service's proto nor its schema exposed a way
-- to ask this question — see this service's README.
ALTER TABLE workflow.executions ADD COLUMN project_id UUID;

-- Partial index scoped to the non-terminal statuses HasActiveExecutions
-- filters on (pending/running/paused) — mirrors
-- idx_workflow_executions_resumable's "only index the rows a real query
-- needs" pattern from 0001_init.
CREATE INDEX idx_workflow_executions_project_active ON workflow.executions (tenant_id, project_id, status)
    WHERE status IN ('pending', 'running', 'paused');
