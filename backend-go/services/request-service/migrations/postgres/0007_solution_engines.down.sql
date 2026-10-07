UPDATE request.analysis_runs SET mode='complete' WHERE mode='agent_proposal';
ALTER TABLE request.analysis_runs DROP CONSTRAINT IF EXISTS analysis_runs_mode_check;
ALTER TABLE request.analysis_runs ADD CONSTRAINT analysis_runs_mode_check CHECK (mode IN ('complete','agent_readonly'));
ALTER TABLE request.analysis_runs DROP COLUMN engine;
-- chỉ dùng khi rollback toàn bộ v6
ALTER TABLE request.requests DROP COLUMN solution_engine;

DROP TABLE request.openspec_changes;
DROP TABLE request.project_engine_settings;
