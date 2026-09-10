ALTER TABLE infra.dev_servers
  ADD COLUMN status TEXT NOT NULL DEFAULT 'pending'
    CHECK (status IN ('pending','healthy','degraded','unhealthy')),
  ADD COLUMN platform TEXT, ADD COLUMN arch TEXT,
  ADD COLUMN node_version TEXT, ADD COLUMN agent_version TEXT,
  ADD COLUMN last_provisioned_at TIMESTAMPTZ;
