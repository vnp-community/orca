DROP INDEX IF EXISTS automation.idx_automations_project;

ALTER TABLE automation.automations
  DROP COLUMN project_id;
