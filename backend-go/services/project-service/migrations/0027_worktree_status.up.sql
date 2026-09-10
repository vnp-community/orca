-- status is orthogonal to `active` (activation_state) — see
-- domain.WorktreeStatus's doc comment. Backs BL-AT-04's
-- cleanup_worktrees step (status_in/older_than filters, see
-- ListWorktrees below), not any activation concept.
ALTER TABLE project.worktrees
  ADD COLUMN status TEXT NOT NULL DEFAULT 'active'
  CHECK (status IN ('active', 'completed', 'error', 'stopped'));
