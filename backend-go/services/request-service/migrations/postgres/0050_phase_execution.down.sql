DROP INDEX request.requests_executing_scan;
DROP POLICY relay_scan ON request.requests;
DROP TABLE request.execution_reconcile_state;
DROP TABLE request.task_run_outcomes;
DROP TABLE request.phase_starts;
