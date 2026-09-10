-- worktree_id/result/error_message/reported_at close the lifecycle gap
-- BUG-TASKV1-005 identifies: StartCoordinatorRun/CompleteCoordinatorRun/
-- FailCoordinatorRun all need a column to write into that doesn't exist
-- yet. reported_at additionally lets a retry pass (see
-- TASK-TASKV1-005-07) find a run whose terminal state was persisted but
-- never successfully reported back to task-service.
ALTER TABLE orchestration.coordinator_runs
  ADD COLUMN worktree_id   TEXT,        -- optional, same nullable pattern as dispatch_contexts.worktree_id (0003)
  ADD COLUMN result        JSONB,       -- set by CompleteCoordinatorRun / the UpdateTaskStatusAndPromote run-finalization tail
  ADD COLUMN error_message TEXT,        -- set by FailCoordinatorRun
  ADD COLUMN reported_at   TIMESTAMPTZ; -- set once TaskServiceReporter.ReportResult succeeds

CREATE INDEX idx_coordinator_runs_unreported
  ON orchestration.coordinator_runs (status)
  WHERE status IN ('completed', 'failed') AND reported_at IS NULL;
