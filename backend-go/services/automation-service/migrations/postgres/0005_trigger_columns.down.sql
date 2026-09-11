DROP INDEX IF EXISTS automation.idx_automations_trigger;

ALTER TABLE automation.automations
  DROP COLUMN trigger_filter_json,
  DROP COLUMN trigger_event,
  DROP COLUMN trigger_type;
