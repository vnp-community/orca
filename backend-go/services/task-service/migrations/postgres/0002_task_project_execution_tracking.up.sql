-- Epic C (backend-go/docs/execution-plan.md): adds the plumbing
-- usecase.HasActiveExecutions needs to answer "does this project have a
-- task currently dispatched for execution" — closing
-- project-service.RebindDevServer's active-execution guard, which was a
-- no-op because neither task-service's nor workflow-service's proto exposed
-- this query before now.
--
-- HONEST CAVEAT (see internal/usecase/execute_task.go and
-- has_active_executions.go's doc comments, and this service's README):
-- task-service has no execution-completion callback in this scaffold, and
-- the generated proto has no UpdateTask/SetStatus RPC to drive one
-- manually. That means status = 'in_progress' is a real, persisted state
-- transition (set once, on ExecuteTask dispatch) but it is currently
-- ONE-WAY — nothing ever clears it back out. A query against this index
-- will therefore over-report "active" for any task ever executed, until a
-- real completion path is built as separate, later work.
ALTER TABLE task.tasks ADD COLUMN project_id UUID;

CREATE INDEX idx_tasks_project_active ON task.tasks (tenant_id, project_id, status)
    WHERE status = 'in_progress';
