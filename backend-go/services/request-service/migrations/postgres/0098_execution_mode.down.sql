ALTER TABLE request.phase_starts DROP COLUMN IF EXISTS start_trigger;
ALTER TABLE request.requests DROP COLUMN IF EXISTS execution_mode;
ALTER TABLE request.project_engine_settings DROP COLUMN IF EXISTS execution_mode;
