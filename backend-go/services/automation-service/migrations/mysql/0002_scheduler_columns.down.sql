ALTER TABLE automation_runs DROP COLUMN IF EXISTS `trigger`;

DROP INDEX idx_automations_due ON automations;

ALTER TABLE automations
    DROP COLUMN IF EXISTS next_run_at,
    DROP COLUMN IF EXISTS timezone,
    DROP COLUMN IF EXISTS enabled,
    DROP COLUMN IF EXISTS step_type;
