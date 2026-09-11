-- MySQL/TiDB variant of postgres/0002_dispatch_context_user_id.up.sql.
-- VARCHAR(255), not TEXT: user_id is part of idx_dispatch_contexts_user
-- below — same InnoDB TEXT-in-key restriction as handle in 0001_init.
ALTER TABLE dispatch_contexts
  ADD COLUMN user_id VARCHAR(255) NULL;

-- Postgres's idx_dispatch_contexts_user is a PARTIAL index
-- (WHERE user_id IS NOT NULL) — MySQL has no filtered index, so this is a
-- plain composite index over the same columns (same trade-off as
-- idx_gates_pending in 0001_init).
CREATE INDEX idx_dispatch_contexts_user ON dispatch_contexts (tenant_id, user_id);
