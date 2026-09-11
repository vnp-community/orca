-- VARCHAR, not TEXT, for this CHECK-constrained enum column — see
-- migrations/mysql/0013_ephemeral_vm_runtimes.up.sql's comment.
ALTER TABLE dev_servers
  ADD COLUMN status VARCHAR(16) NOT NULL DEFAULT 'pending'
    CHECK (status IN ('pending','healthy','degraded','unhealthy')),
  ADD COLUMN platform TEXT, ADD COLUMN arch TEXT,
  ADD COLUMN node_version TEXT, ADD COLUMN agent_version TEXT,
  ADD COLUMN last_provisioned_at TIMESTAMP(6);
