UPDATE analysis_runs SET mode='complete' WHERE mode='agent_proposal';
ALTER TABLE analysis_runs DROP CHECK analysis_runs_mode_check;
ALTER TABLE analysis_runs ADD CONSTRAINT analysis_runs_chk_1 CHECK (mode IN ('complete','agent_readonly'));
ALTER TABLE analysis_runs DROP COLUMN engine;
-- chỉ dùng khi rollback toàn bộ v6
ALTER TABLE requests DROP COLUMN solution_engine;

DROP TABLE openspec_changes;
DROP TABLE project_engine_settings;
