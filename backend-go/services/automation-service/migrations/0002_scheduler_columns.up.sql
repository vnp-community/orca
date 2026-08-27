-- Promotes step_type off the step_config_json blob to a real column, and
-- adds the columns the in-process scheduler ticker needs — see
-- specs/backend-go/services/automation-service.md §5/§7. dtstart already
-- exists (migration 0001); enabled/timezone/next_run_at are new here.
ALTER TABLE automation.automations
    ADD COLUMN step_type   TEXT NOT NULL DEFAULT '',
    ADD COLUMN enabled     BOOLEAN NOT NULL DEFAULT true,
    ADD COLUMN timezone    TEXT NOT NULL DEFAULT 'UTC',
    ADD COLUMN next_run_at TIMESTAMPTZ;

-- Partial index matching the scheduler's due-row query
-- (WHERE enabled = true AND next_run_at <= now()), per automation-service.md §7.
CREATE INDEX idx_automations_due ON automation.automations (next_run_at) WHERE enabled;

-- trigger records what caused a run's dispatch (scheduled/manual/external)
-- — see automation-service.md §3/§7: RunNow, the scheduler ticker, and
-- HandleExternalTrigger all funnel through the same interactor, and this
-- is how a run's origin is told apart afterward.
ALTER TABLE automation.automation_runs
    ADD COLUMN trigger TEXT NOT NULL DEFAULT 'manual'
        CHECK (trigger IN ('scheduled', 'manual', 'external'));
