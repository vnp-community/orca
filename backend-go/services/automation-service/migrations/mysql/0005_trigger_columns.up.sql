-- BL-AT-03's trigger schema: cron (default, back-compat) | manual | event.
-- trigger_event/trigger_filter_json are only meaningful when
-- trigger_type = 'event' — see domain.NewAutomation's validation.
-- trigger_filter_json is JSON (Postgres: JSONB, see 0003's comment for the
-- JSONB->JSON rule).
-- MySQL/InnoDB rejects a plain literal DEFAULT on a TEXT column (see
-- 0001_init.up.sql's comment on the same restriction) — wrapped in
-- parentheses as an expression default instead.
ALTER TABLE automations
  ADD COLUMN trigger_type TEXT NOT NULL DEFAULT ('cron'),
  ADD COLUMN trigger_event TEXT,
  ADD COLUMN trigger_filter_json JSON;

-- Postgres's idx_automations_trigger is a PARTIAL index
-- (WHERE trigger_type = 'event') — no MySQL equivalent, full index instead
-- (performance-only deviation, same category as 0002's idx_automations_due
-- — see that migration's comment). trigger_type is TEXT (unbounded, like
-- Postgres), so the composite index needs an explicit prefix length on it,
-- matching annotation-service's file_path/repo_id precedent
-- (BE-DB-SOL-005 §5).
CREATE INDEX idx_automations_trigger ON automations (tenant_id, trigger_type(16), trigger_event(255));
