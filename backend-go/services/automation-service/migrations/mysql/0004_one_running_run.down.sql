DROP INDEX idx_automation_runs_one_running ON automation_runs;
ALTER TABLE automation_runs DROP COLUMN running_slot;
