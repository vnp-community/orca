-- VARCHAR, not TEXT, for this CHECK-constrained enum column — see
-- migrations/mysql/0013_ephemeral_vm_runtimes.up.sql's comment.
ALTER TABLE fleet_health
  ADD COLUMN status VARCHAR(16) NOT NULL DEFAULT 'unreachable'
    CHECK (status IN ('healthy','degraded','unhealthy','unreachable'));
