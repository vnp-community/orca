ALTER TABLE worktrees
    ADD COLUMN parent_worktree_id CHAR(36),
    ADD COLUMN origin TEXT,
    ADD COLUMN capture_source TEXT,
    ADD COLUMN capture_confidence TEXT,
    ADD COLUMN task_id TEXT,
    ADD COLUMN orchestration_run_id TEXT,
    ADD COLUMN coordinator_handle TEXT,
    ADD COLUMN created_by_terminal_handle TEXT;

ALTER TABLE worktrees
    ADD CONSTRAINT fk_worktrees_parent_worktree FOREIGN KEY (parent_worktree_id) REFERENCES worktrees (id) ON DELETE SET NULL;

-- Postgres source is a PARTIAL index (`WHERE parent_worktree_id IS NOT
-- NULL`) — MySQL has no partial index, so this is a full index instead
-- (annotation-service's BE-DB-SOL-005 §5 precedent for this exact gap).
-- Slightly larger than the Postgres index (includes NULL entries too),
-- functionally equivalent for ListLineage's `WHERE parent_worktree_id IS
-- NOT NULL` query.
CREATE INDEX idx_worktrees_parent_worktree ON worktrees (parent_worktree_id);
