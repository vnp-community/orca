DROP POLICY relay_claim ON request.analysis_runs;
DROP POLICY relay_scan ON request.analysis_runs;
DROP TABLE request.analysis_project_gates;
DROP INDEX request.analysis_runs_project_running;
ALTER TABLE request.analysis_runs
    DROP COLUMN repo_check,
    DROP COLUMN enforcement,
    DROP COLUMN feedback,
    DROP COLUMN actor_id,
    DROP COLUMN project_id,
    DROP COLUMN solution_id;
DROP INDEX request.idx_solutions_request_state;
ALTER TABLE request.solutions
    DROP COLUMN generation_run_id,
    DROP COLUMN content_ref,
    DROP COLUMN status,
    DROP COLUMN kind;
