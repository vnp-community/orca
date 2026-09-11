-- Adds inputs_json to workflow.executions — TASK-WF-003-01's
-- ExecuteRequest.inputs, frozen at Execute time so a step dispatched after
-- a restart (RecoverExecutions' boot-time scan) still has the original
-- {{input_field}} values available for usecase.Interpolate.
ALTER TABLE workflow.executions ADD COLUMN inputs_json JSONB;
