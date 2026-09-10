DROP INDEX IF EXISTS project.worktrees_project_idempotency_key_idx;
ALTER TABLE project.worktrees DROP COLUMN IF EXISTS idempotency_key;
