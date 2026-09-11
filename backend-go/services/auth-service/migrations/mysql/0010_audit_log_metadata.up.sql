-- ip_address was already added by 0004_audit_outcome_and_ip.
--
-- metadata has no DEFAULT ('{}') here (unlike the Postgres variant's
-- `DEFAULT '{}'::jsonb`): Append() always marshals entry.Metadata and
-- includes it in the INSERT column list on every write path, so the
-- default is never actually read by the application — and a MySQL-dialect
-- deployment of this service starts from migration 0001 onward, so this
-- ALTER always runs against an empty (or already-metadata-less-by-design)
-- table, with nothing that needs the NOT NULL constraint satisfied
-- retroactively (mirrors annotation-service's BE-DB-SOL-005 §"MySQL
-- variant" reasoning for the same "fresh deployment, no backfill needed"
-- situation).
ALTER TABLE audit_log
  ADD COLUMN target_type VARCHAR(64),
  ADD COLUMN target_id   VARCHAR(255),
  ADD COLUMN metadata    JSON NOT NULL;

-- Backfill: split the existing `target` column on the pre-existing
-- action-name convention (user.* actions target a user, session.* actions
-- target a session) — best-effort, historical rows may have target_type
-- left NULL where the action name doesn't map cleanly. Mirrors the
-- Postgres variant's split_part(action, '.', 1) using
-- SUBSTRING_INDEX(action, '.', 1), MySQL's equivalent.
UPDATE audit_log SET
  target_type = SUBSTRING_INDEX(action, '.', 1),
  target_id   = target
WHERE target_type IS NULL;

CREATE INDEX idx_audit_log_action ON audit_log (action);
CREATE INDEX idx_audit_log_target ON audit_log (target_type, target_id);
