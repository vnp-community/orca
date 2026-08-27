ALTER TABLE automation.automation_runs
  DROP COLUMN action_results_json;

DROP INDEX IF EXISTS automation.idx_automations_project;

ALTER TABLE automation.automations
  DROP COLUMN actions_json,
  DROP COLUMN project_id;
