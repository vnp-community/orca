-- CR-REQ-044: Orca starts the Plan of a Request itself once its Analysis is approved. One job per cause keeps the state
-- (attempts, schedule, outcome) so it survives a restart and the Plan tab can show it after a reload.
CREATE TABLE request.plan_auto_jobs (
    tenant_id UUID NOT NULL,
    request_id UUID NOT NULL REFERENCES request.requests(id),
    anchor TEXT NOT NULL,
    origin TEXT NOT NULL CHECK (origin IN ('event','retry')),
    project_id UUID NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('pending','running','committing','done','gave_up','skipped','superseded')),
    attempt INT NOT NULL DEFAULT 1,
    max_attempts INT NOT NULL DEFAULT 1,
    run_id UUID NULL,
    reason_code TEXT NULL,
    commit_attempts INT NOT NULL DEFAULT 0,
    next_check_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    lease_owner TEXT NULL,
    lease_expires_at TIMESTAMPTZ NULL,
    requested_by UUID NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, request_id, anchor)
);

CREATE UNIQUE INDEX plan_auto_jobs_one_active ON request.plan_auto_jobs (tenant_id, request_id)
    WHERE state IN ('pending','running','committing');
CREATE INDEX plan_auto_jobs_due ON request.plan_auto_jobs (next_check_at)
    WHERE state IN ('pending','running','committing');
CREATE INDEX plan_auto_jobs_by_request ON request.plan_auto_jobs (tenant_id, request_id, created_at DESC);

ALTER TABLE request.plan_auto_jobs ENABLE ROW LEVEL SECURITY;
ALTER TABLE request.plan_auto_jobs FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON request.plan_auto_jobs
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

-- The worker claims due jobs across tenants, like the analysis-run sweeper.
CREATE POLICY relay_scan ON request.plan_auto_jobs
    AS PERMISSIVE FOR SELECT
    USING (current_setting('app.relay', true) = 'on');
CREATE POLICY relay_claim ON request.plan_auto_jobs
    AS PERMISSIVE FOR UPDATE
    USING (current_setting('app.relay', true) = 'on')
    WITH CHECK (current_setting('app.relay', true) = 'on');

-- On unless a project turns it off; a project that never saved settings has no row and reads as on.
ALTER TABLE request.project_engine_settings
    ADD COLUMN plan_auto_generate BOOLEAN NOT NULL DEFAULT true;
