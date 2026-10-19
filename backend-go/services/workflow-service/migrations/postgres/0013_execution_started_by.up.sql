-- The user who started the execution. Steps that resolve a project target call project-service, which is
-- membership-gated, so a run recovered after a restart needs the same acting user it started with.
ALTER TABLE workflow.executions ADD COLUMN started_by TEXT;
