-- orca_mcp_killswitch_active is sampled across tenants by the metrics worker.
-- Read-only and only active rows; it opts in per transaction via app.relay like
-- the other background workers.
CREATE POLICY worker_count_active ON mcp.kill_switches FOR SELECT
    USING (current_setting('app.relay', true) = 'on' AND active);
