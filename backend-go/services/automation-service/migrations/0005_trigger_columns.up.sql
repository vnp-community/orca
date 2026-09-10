-- BL-AT-03's trigger schema: cron (default, back-compat) | manual | event.
-- trigger_event/trigger_filter_json are only meaningful when
-- trigger_type = 'event' — see domain.NewAutomation's validation.
ALTER TABLE automation.automations
  ADD COLUMN trigger_type TEXT NOT NULL DEFAULT 'cron',
  ADD COLUMN trigger_event TEXT,
  ADD COLUMN trigger_filter_json JSONB;

CREATE INDEX idx_automations_trigger ON automation.automations (tenant_id, trigger_type, trigger_event)
  WHERE trigger_type = 'event';
