-- BE-SOL-002 (TASK-FT-002-02): logical FK back to task-service.Task.id for
-- an execution started via Engine 3 dispatch (task-service's
-- WorkflowExecutor, TASK-FT-002-03) — empty for a standalone workflow run.
-- NOT NULL DEFAULT '' (not nullable) matches this solution's "empty =
-- standalone workflow run" convention (runToCompletion's later check is
-- `if exec.OriginTaskID != ""`, TASK-FT-002-05) and needs no backfill
-- migration for existing rows.
ALTER TABLE workflow.executions ADD COLUMN origin_task_id TEXT NOT NULL DEFAULT '';
