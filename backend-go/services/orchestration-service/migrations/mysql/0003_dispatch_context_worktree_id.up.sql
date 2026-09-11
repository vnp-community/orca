-- MySQL/TiDB variant of postgres/0003_dispatch_context_worktree_id.up.sql.
-- VARCHAR(255), not TEXT: worktree_id is part of
-- idx_dispatch_contexts_worktree below — same InnoDB TEXT-in-key
-- restriction as handle/user_id above.
ALTER TABLE dispatch_contexts
  ADD COLUMN worktree_id VARCHAR(255) NULL;

-- Plain composite index — see 0002's comment on the same partial->plain
-- index trade-off.
CREATE INDEX idx_dispatch_contexts_worktree ON dispatch_contexts (tenant_id, worktree_id);
