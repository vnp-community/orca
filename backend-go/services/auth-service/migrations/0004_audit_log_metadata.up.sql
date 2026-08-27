ALTER TABLE auth.audit_log
  ADD COLUMN target_type TEXT,
  ADD COLUMN target_id   TEXT,
  ADD COLUMN metadata    JSONB NOT NULL DEFAULT '{}'::jsonb,
  ADD COLUMN ip_address  INET;

-- Backfill: split the existing `target` column on the pre-existing
-- action-name convention (user.* actions target a user, session.* actions
-- target a session) — best-effort, historical rows may have target_type
-- left NULL where the action name doesn't map cleanly.
UPDATE auth.audit_log SET
  target_type = split_part(action, '.', 1),
  target_id   = target
WHERE target_type IS NULL;

CREATE INDEX IF NOT EXISTS idx_audit_log_action ON auth.audit_log (action);
CREATE INDEX IF NOT EXISTS idx_audit_log_target ON auth.audit_log (target_type, target_id);
