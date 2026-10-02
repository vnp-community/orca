-- See the Postgres variant. MySQL has no partial indexes, so the keyset index
-- leads with actor_type instead.
ALTER TABLE audit_log
  ADD COLUMN actor_type VARCHAR(16) NOT NULL DEFAULT 'user',
  ADD CONSTRAINT audit_log_actor_type_check CHECK (actor_type IN ('user', 'agent', 'system'));

CREATE INDEX idx_audit_log_actor_type_time ON audit_log (tenant_id, actor_type, occurred_at, id);
