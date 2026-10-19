-- How a Request runs once its plan is approved. step_by_step is the original behaviour (a person releases every Phase);
-- continuous lets Orca start the next Phase itself. The Request pins the mode when it enters executing, so changing the
-- project setting never changes the rules of a Request that is already running.
ALTER TABLE project_engine_settings
    ADD COLUMN execution_mode VARCHAR(20) NOT NULL DEFAULT 'step_by_step',
    ADD CONSTRAINT project_engine_settings_execution_mode CHECK (execution_mode IN ('step_by_step', 'continuous'));

ALTER TABLE requests
    ADD COLUMN execution_mode VARCHAR(20) NULL,
    ADD CONSTRAINT requests_execution_mode CHECK (execution_mode IS NULL OR execution_mode IN ('step_by_step', 'continuous'));

-- 'auto' marks a Phase Orca started itself, so the activity feed can tell it from a button a person pressed.
ALTER TABLE phase_starts
    ADD COLUMN start_trigger VARCHAR(10) NOT NULL DEFAULT 'manual',
    ADD CONSTRAINT phase_starts_start_trigger CHECK (start_trigger IN ('manual', 'auto'));
