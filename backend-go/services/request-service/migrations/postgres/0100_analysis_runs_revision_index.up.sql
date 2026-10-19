-- The activity feed reads the analysis runs of one Request (CR-REQ-041). The existing idempotency index is partial
-- (idempotency_key IS NOT NULL), so without this one that read scans every run of every tenant. MySQL needs no counterpart:
-- its analysis_runs_idem unique key is not partial and already serves the (tenant_id, request_id) prefix.
CREATE INDEX analysis_runs_by_request ON request.analysis_runs (tenant_id, request_id, started_at);
