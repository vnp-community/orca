-- Intentionally empty: the Postgres 0100 adds analysis_runs_by_request, but MySQL's unique key analysis_runs_idem
-- (tenant_id, request_id, ...) is not partial and already serves that read. The file keeps both dialects on the same number.
SELECT 1;
