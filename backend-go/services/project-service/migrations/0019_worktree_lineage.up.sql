-- Worktree lineage — explicit-capture only for now (see
-- proto/orca/project/v1/project.proto's WorktreeLineageEntry doc comment).
-- Additive columns on the existing project.worktrees table (0004), same
-- convention: nullable everywhere since most worktrees are never branched
-- from another one and never carry this context.
ALTER TABLE project.worktrees
    ADD COLUMN parent_worktree_id UUID REFERENCES project.worktrees (id) ON DELETE SET NULL,
    ADD COLUMN origin TEXT,
    ADD COLUMN capture_source TEXT,
    ADD COLUMN capture_confidence TEXT,
    ADD COLUMN task_id TEXT,
    ADD COLUMN orchestration_run_id TEXT,
    ADD COLUMN coordinator_handle TEXT,
    ADD COLUMN created_by_terminal_handle TEXT;

CREATE INDEX idx_worktrees_parent_worktree ON project.worktrees (parent_worktree_id) WHERE parent_worktree_id IS NOT NULL;
