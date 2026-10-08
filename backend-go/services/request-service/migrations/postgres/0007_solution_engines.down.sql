ALTER TABLE request.analysis_runs DROP COLUMN IF EXISTS engine;
ALTER TABLE request.requests DROP COLUMN IF EXISTS solution_engine;

DROP TABLE IF EXISTS request.openspec_changes;
DROP TABLE IF EXISTS request.project_engine_settings;
