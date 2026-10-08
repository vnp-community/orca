ALTER TABLE analysis_runs DROP COLUMN engine;
ALTER TABLE requests DROP COLUMN solution_engine;

DROP TABLE openspec_changes;
DROP TABLE project_engine_settings;
