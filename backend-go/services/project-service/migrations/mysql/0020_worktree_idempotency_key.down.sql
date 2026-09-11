DROP INDEX worktrees_project_idempotency_key_idx ON worktrees;
ALTER TABLE worktrees DROP COLUMN idempotency_key;
