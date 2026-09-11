ALTER TABLE worktrees
  ADD COLUMN status VARCHAR(16) NOT NULL DEFAULT 'active'
  CHECK (status IN ('active', 'completed', 'error', 'stopped'));
