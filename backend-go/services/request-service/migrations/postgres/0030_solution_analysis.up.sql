-- Solution kind/status live beside the foundation columns; draft is the placeholder an async run fills.
ALTER TABLE request.solutions
    ADD COLUMN kind TEXT NOT NULL DEFAULT 'solution' CHECK (kind IN ('solution','diagnosis','findings','answer')),
    ADD COLUMN status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','proposed','approved','rejected','superseded')),
    ADD COLUMN content_ref TEXT NOT NULL DEFAULT '',
    ADD COLUMN generation_run_id UUID NULL;

CREATE INDEX idx_solutions_request_state ON request.solutions (tenant_id, request_id, kind, status);

-- A run needs everything its worker reads back after an RPC returned or the process restarted.
ALTER TABLE request.analysis_runs
    ADD COLUMN solution_id UUID NULL,
    ADD COLUMN project_id UUID NULL,
    ADD COLUMN actor_id UUID NULL,
    ADD COLUMN feedback TEXT NULL,
    ADD COLUMN enforcement TEXT NULL CHECK (enforcement IN ('agent_enforced','prompt_only')),
    ADD COLUMN repo_check TEXT NULL CHECK (repo_check IN ('skipped','clean','modified'));

CREATE INDEX analysis_runs_project_running ON request.analysis_runs (tenant_id, project_id, mode) WHERE status = 'running';

-- One row per project is the lock that serialises the agent_readonly concurrency check (no partial-index counting race).
CREATE TABLE request.analysis_project_gates (
    tenant_id UUID NOT NULL,
    project_id UUID NOT NULL,
    PRIMARY KEY (tenant_id, project_id)
);

ALTER TABLE request.analysis_project_gates ENABLE ROW LEVEL SECURITY;
ALTER TABLE request.analysis_project_gates FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON request.analysis_project_gates
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

-- Crash recovery scans expired leases across tenants, like classification_runs and the outbox relay.
CREATE POLICY relay_scan ON request.analysis_runs
    AS PERMISSIVE FOR SELECT
    USING (current_setting('app.relay', true) = 'on');
CREATE POLICY relay_claim ON request.analysis_runs
    AS PERMISSIVE FOR UPDATE
    USING (current_setting('app.relay', true) = 'on')
    WITH CHECK (current_setting('app.relay', true) = 'on');
