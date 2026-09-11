DROP INDEX idx_automations_project ON automations;

ALTER TABLE automations
  DROP COLUMN project_id;
