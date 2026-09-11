-- BR-AT-14: a real per-worktree, per-reason audit trail for
-- workflow-service's CleanupWorktreesStepExecutor — one row per worktree
-- per cleanup run, not just the aggregate counts already in
-- automation_runs.output_json. Written via the reverse-direction
-- WriteCleanupReport RPC (workflow-service -> automation-service).
CREATE TABLE automation.worktree_cleanup_log (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id      UUID NOT NULL,
  run_id         UUID NOT NULL REFERENCES automation.automation_runs (id) ON DELETE CASCADE,
  worktree_id    TEXT NOT NULL,
  action         TEXT NOT NULL CHECK (action IN ('deleted','skipped','would_delete')),
  reason         TEXT,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_worktree_cleanup_log_run ON automation.worktree_cleanup_log (run_id);

ALTER TABLE automation.worktree_cleanup_log ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON automation.worktree_cleanup_log
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
