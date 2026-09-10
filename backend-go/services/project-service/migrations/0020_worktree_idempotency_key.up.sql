ALTER TABLE project.worktrees ADD COLUMN idempotency_key TEXT;

-- Unique per project, not global — the same idempotency_key value is
-- meaningless across two different projects (orca-cli scopes its default
-- key as sha256(project_id|repo_id|branch), which already includes
-- project_id, but a caller-supplied custom key might not).
CREATE UNIQUE INDEX worktrees_project_idempotency_key_idx
  ON project.worktrees (project_id, idempotency_key)
  WHERE idempotency_key IS NOT NULL;
