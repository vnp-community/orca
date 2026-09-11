DROP INDEX idx_worktrees_parent_worktree ON worktrees;
ALTER TABLE worktrees DROP FOREIGN KEY fk_worktrees_parent_worktree;

ALTER TABLE worktrees
    DROP COLUMN IF EXISTS parent_worktree_id,
    DROP COLUMN IF EXISTS origin,
    DROP COLUMN IF EXISTS capture_source,
    DROP COLUMN IF EXISTS capture_confidence,
    DROP COLUMN IF EXISTS task_id,
    DROP COLUMN IF EXISTS orchestration_run_id,
    DROP COLUMN IF EXISTS coordinator_handle,
    DROP COLUMN IF EXISTS created_by_terminal_handle;
