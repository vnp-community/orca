-- Idempotency key of "start this Phase": pressing the button twice must not dispatch twice.
CREATE TABLE request.phase_starts (
    tenant_id UUID NOT NULL,
    phase_task_id UUID NOT NULL,
    request_id UUID NOT NULL,
    started_by UUID NOT NULL,
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, phase_task_id)
);
CREATE INDEX phase_starts_request ON request.phase_starts (tenant_id, request_id);

-- One row per task event we acted on. event_id is unique so a redelivered event cannot count a failure twice.
CREATE TABLE request.task_run_outcomes (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    request_id UUID NOT NULL,
    task_id UUID NOT NULL,
    container_id UUID NULL,
    outcome TEXT NOT NULL CHECK (outcome IN ('started','succeeded','failed','cancelled','phase_done','plan_done')),
    cause TEXT NOT NULL DEFAULT '',
    execution_link_id UUID NULL,
    error_message TEXT NOT NULL DEFAULT '',
    event_id UUID NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    -- 1 for phase_done/plan_done, NULL otherwise: NULLs never collide, so a container completes once.
    once SMALLINT NULL CHECK (once = 1)
);
CREATE UNIQUE INDEX task_run_outcomes_event ON request.task_run_outcomes (tenant_id, event_id);
CREATE UNIQUE INDEX task_run_outcomes_once ON request.task_run_outcomes (tenant_id, task_id, once);
CREATE INDEX idx_task_run_outcomes_request ON request.task_run_outcomes (tenant_id, request_id, occurred_at);
CREATE INDEX idx_task_run_outcomes_task ON request.task_run_outcomes (tenant_id, task_id, outcome);

-- Cross-replica lease for the reconcile loop; last_run_at throttles re-evaluating a request that is simply waiting on a human.
CREATE TABLE request.execution_reconcile_state (
    tenant_id UUID NOT NULL,
    request_id UUID NOT NULL,
    lease_owner TEXT NOT NULL DEFAULT '',
    lease_until TIMESTAMPTZ NOT NULL DEFAULT '2000-01-01',
    last_run_at TIMESTAMPTZ NULL,
    PRIMARY KEY (tenant_id, request_id)
);

DO $$
DECLARE
    t TEXT;
BEGIN
    FOREACH t IN ARRAY ARRAY['phase_starts','task_run_outcomes','execution_reconcile_state'] LOOP
        EXECUTE 'ALTER TABLE request.' || t || ' ENABLE ROW LEVEL SECURITY';
        EXECUTE 'ALTER TABLE request.' || t || ' FORCE ROW LEVEL SECURITY';
        EXECUTE 'CREATE POLICY tenant_isolation ON request.' || t || '
            AS PERMISSIVE FOR ALL TO PUBLIC
            USING (tenant_id = NULLIF(current_setting(''app.tenant_id'', true), '''')::uuid)
            WITH CHECK (tenant_id = NULLIF(current_setting(''app.tenant_id'', true), '''')::uuid)';
    END LOOP;
END $$;

-- The reconcile loop looks for quiet executing Requests across tenants (read only, like the approval sweeper);
-- every write after that runs under the Request's own tenant.
CREATE POLICY relay_scan ON request.requests
    AS PERMISSIVE FOR SELECT
    USING (current_setting('app.relay', true) = 'on');
CREATE POLICY relay_scan ON request.task_run_outcomes
    AS PERMISSIVE FOR SELECT
    USING (current_setting('app.relay', true) = 'on');
CREATE POLICY relay_scan ON request.execution_reconcile_state
    AS PERMISSIVE FOR SELECT
    USING (current_setting('app.relay', true) = 'on');
CREATE INDEX requests_executing_scan ON request.requests (updated_at) WHERE status = 'executing';
