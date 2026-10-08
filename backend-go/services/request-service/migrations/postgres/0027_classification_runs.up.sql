CREATE TABLE request.classification_runs (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    request_id UUID NOT NULL REFERENCES request.requests(id),
    trigger_name TEXT NOT NULL,
    source_event_id UUID NULL,
    manual BOOLEAN NOT NULL DEFAULT false,
    actor_id UUID NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('running','succeeded','failed')),
    claims INT NOT NULL DEFAULT 1,
    lease_owner TEXT NOT NULL DEFAULT '',
    lease_expires_at TIMESTAMPTZ NOT NULL,
    error_code TEXT NOT NULL DEFAULT '',
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ NULL,
    -- 1 while running, NULL after: NULLs never collide, so the unique index allows one live run per request.
    active SMALLINT NULL CHECK (active = 1)
);

CREATE UNIQUE INDEX classification_runs_one_running ON request.classification_runs (tenant_id, request_id, active);
CREATE UNIQUE INDEX classification_runs_event ON request.classification_runs (tenant_id, source_event_id) WHERE source_event_id IS NOT NULL;
CREATE INDEX classification_runs_lease_scan ON request.classification_runs (lease_expires_at) WHERE status = 'running';

ALTER TABLE request.classification_runs ENABLE ROW LEVEL SECURITY;
ALTER TABLE request.classification_runs FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON request.classification_runs
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

-- Crash recovery scans expired leases across tenants, like the outbox relay does.
CREATE POLICY relay_scan ON request.classification_runs
    AS PERMISSIVE FOR SELECT
    USING (current_setting('app.relay', true) = 'on');
CREATE POLICY relay_claim ON request.classification_runs
    AS PERMISSIVE FOR UPDATE
    USING (current_setting('app.relay', true) = 'on')
    WITH CHECK (current_setting('app.relay', true) = 'on');

-- Retention sweep of processed_events spans tenants (outbox-style relay access, delete only).
CREATE POLICY relay_prune_scan ON request.processed_events
    AS PERMISSIVE FOR SELECT
    USING (current_setting('app.relay', true) = 'on');
CREATE POLICY relay_prune ON request.processed_events
    AS PERMISSIVE FOR DELETE
    USING (current_setting('app.relay', true) = 'on');
