DROP INDEX idx_coordinator_runs_unreported ON coordinator_runs;
ALTER TABLE coordinator_runs
  DROP COLUMN reported_at,
  DROP COLUMN error_message,
  DROP COLUMN result,
  DROP COLUMN worktree_id;
