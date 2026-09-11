DROP INDEX idx_dispatch_contexts_worktree ON dispatch_contexts;
ALTER TABLE dispatch_contexts
  DROP COLUMN worktree_id;
