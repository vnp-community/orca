-- BR-AT-08: at most one 'running' automation_runs row per automation,
-- enforced with a partial unique index (no read-then-write race window) —
-- mirrors the existing (tenant_id, request_id) idempotency pattern.
CREATE UNIQUE INDEX idx_automation_runs_one_running
  ON automation.automation_runs (automation_id)
  WHERE status = 'running';
