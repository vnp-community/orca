DROP INDEX idx_automations_trigger ON automations;

ALTER TABLE automations
  DROP COLUMN trigger_filter_json,
  DROP COLUMN trigger_event,
  DROP COLUMN trigger_type;
