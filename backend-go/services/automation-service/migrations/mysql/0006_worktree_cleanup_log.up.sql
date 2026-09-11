-- BR-AT-14: a real per-worktree, per-reason audit trail for
-- workflow-service's CleanupWorktreesStepExecutor — one row per worktree
-- per cleanup run, not just the aggregate counts already in
-- automation_runs.output_json. Written via the reverse-direction
-- WriteCleanupReport RPC (workflow-service -> automation-service).
CREATE TABLE worktree_cleanup_log (
  id             CHAR(36) PRIMARY KEY,
  tenant_id      CHAR(36) NOT NULL,
  run_id         CHAR(36) NOT NULL,
  worktree_id    TEXT NOT NULL,
  action         VARCHAR(16) NOT NULL,
  reason         TEXT,
  created_at     TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),

  CONSTRAINT worktree_cleanup_log_action_check CHECK (action IN ('deleted','skipped','would_delete')),
  CONSTRAINT fk_worktree_cleanup_log_run FOREIGN KEY (run_id)
      REFERENCES automation_runs (id) ON DELETE CASCADE
);
CREATE INDEX idx_worktree_cleanup_log_run ON worktree_cleanup_log (run_id);

-- No RLS equivalent — see 0001_init.up.sql's comment; the same policy
-- existed here in Postgres and never actually activated either:
--
--   ALTER TABLE automation.worktree_cleanup_log ENABLE ROW LEVEL SECURITY;
--   CREATE POLICY tenant_isolation ON automation.worktree_cleanup_log
--       USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
