CREATE TABLE request.analysis_runs (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    request_id UUID NOT NULL REFERENCES request.requests(id),
    kind TEXT NOT NULL CHECK (kind IN ('solution','diagnosis','findings','answer')),
    mode TEXT NOT NULL CHECK (mode IN ('complete','agent_readonly','agent_proposal')),
    status TEXT NOT NULL CHECK (status IN ('running','succeeded','failed')),
    idempotency_key TEXT NULL,
    attempt INT NOT NULL DEFAULT 1,
    lease_owner TEXT NULL,
    lease_expires_at TIMESTAMPTZ NULL,
    error_code TEXT NULL,
    error_message TEXT NULL,
    raw_output TEXT NULL,
    engine TEXT NOT NULL DEFAULT 'native',
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ NULL
);

CREATE UNIQUE INDEX analysis_runs_one_running ON request.analysis_runs (tenant_id, request_id, kind) WHERE status='running';
CREATE UNIQUE INDEX analysis_runs_idem ON request.analysis_runs (tenant_id, request_id, idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE INDEX analysis_runs_lease_scan ON request.analysis_runs (status, lease_expires_at);

ALTER TABLE request.analysis_runs ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON request.analysis_runs USING (tenant_id = current_setting('orca.tenant_id', true)::uuid);
