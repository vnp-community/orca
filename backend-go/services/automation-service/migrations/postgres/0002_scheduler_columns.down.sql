ALTER TABLE automation.automation_runs DROP COLUMN IF EXISTS trigger;

DROP INDEX IF EXISTS automation.idx_automations_due;

ALTER TABLE automation.automations
    DROP COLUMN IF EXISTS next_run_at,
    DROP COLUMN IF EXISTS timezone,
    DROP COLUMN IF EXISTS enabled,
    DROP COLUMN IF EXISTS step_type;
