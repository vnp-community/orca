-- Append-only measurements the type policies judge on completion; the newest row per (request, kind) is effective.
CREATE TABLE request.request_checks (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    request_id UUID NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('perf_baseline','perf_after','tests_before','tests_after','security_recheck','ops_result')),
    status TEXT NOT NULL CHECK (status IN ('passed','failed')),
    metrics JSONB NOT NULL DEFAULT '{}'::jsonb,
    summary TEXT NOT NULL DEFAULT '',
    -- orca_verified is reserved for checks Orca measured itself; no RPC may set it.
    source TEXT NOT NULL CHECK (source IN ('agent','manual','orca_verified')),
    task_id UUID NULL,
    recorded_by UUID NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX idx_request_checks_latest ON request.request_checks (tenant_id, request_id, kind, created_at DESC);

ALTER TABLE request.request_checks ENABLE ROW LEVEL SECURITY;
ALTER TABLE request.request_checks FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON request.request_checks
    AS PERMISSIVE FOR ALL TO PUBLIC
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);
