ALTER TABLE phase_starts DROP CONSTRAINT phase_starts_start_trigger, DROP COLUMN start_trigger;
ALTER TABLE requests DROP CONSTRAINT requests_execution_mode, DROP COLUMN execution_mode;
ALTER TABLE project_engine_settings DROP CONSTRAINT project_engine_settings_execution_mode, DROP COLUMN execution_mode;
