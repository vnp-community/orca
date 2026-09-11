-- MySQL/TiDB variant of postgres/0004_coordinator_run_lifecycle.up.sql.
ALTER TABLE coordinator_runs
  ADD COLUMN worktree_id   VARCHAR(255), -- optional, same nullable pattern as dispatch_contexts.worktree_id (0003)
  ADD COLUMN result        JSON,         -- set by CompleteCoordinatorRun / the UpdateTaskStatusAndPromote run-finalization tail
  ADD COLUMN error_message TEXT,         -- set by FailCoordinatorRun
  ADD COLUMN reported_at   TIMESTAMP(6) NULL; -- set once TaskServiceReporter.ReportResult succeeds

-- Postgres's idx_coordinator_runs_unreported is a PARTIAL index
-- (WHERE status IN ('completed','failed') AND reported_at IS NULL) —
-- MySQL has no filtered index, so this is a plain composite index over the
-- same columns (same trade-off documented in 0001_init/0002/0003 above).
CREATE INDEX idx_coordinator_runs_unreported ON coordinator_runs (status, reported_at);
