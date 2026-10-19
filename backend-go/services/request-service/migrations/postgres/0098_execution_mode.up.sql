-- How a Request runs once its plan is approved. step_by_step is the original behaviour (a person releases every Phase);
-- continuous lets Orca start the next Phase itself. The Request pins the mode when it enters executing, so changing the
-- project setting never changes the rules of a Request that is already running.
ALTER TABLE request.project_engine_settings
    ADD COLUMN execution_mode TEXT NOT NULL DEFAULT 'step_by_step'
        CHECK (execution_mode IN ('step_by_step', 'continuous'));

ALTER TABLE request.requests
    ADD COLUMN execution_mode TEXT NULL
        CHECK (execution_mode IS NULL OR execution_mode IN ('step_by_step', 'continuous'));

-- 'auto' marks a Phase Orca started itself, so the activity feed can tell it from a button a person pressed.
ALTER TABLE request.phase_starts
    ADD COLUMN start_trigger TEXT NOT NULL DEFAULT 'manual'
        CHECK (start_trigger IN ('manual', 'auto'));
