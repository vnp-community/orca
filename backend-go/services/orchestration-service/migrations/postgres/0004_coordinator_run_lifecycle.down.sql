DROP INDEX IF EXISTS orchestration.idx_coordinator_runs_unreported;
ALTER TABLE orchestration.coordinator_runs
  DROP COLUMN IF EXISTS reported_at,
  DROP COLUMN IF EXISTS error_message,
  DROP COLUMN IF EXISTS result,
  DROP COLUMN IF EXISTS worktree_id;
