CREATE TABLE request.project_engine_settings (
    tenant_id UUID NOT NULL,
    project_id UUID NOT NULL,
    solution_engine TEXT NOT NULL CHECK (solution_engine IN ('native','openspec')),
    openspec_min_version TEXT NULL,
    updated_by UUID NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    version BIGINT NOT NULL DEFAULT 1,
    PRIMARY KEY (tenant_id, project_id)
);

CREATE TABLE request.openspec_changes (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    request_id UUID NOT NULL REFERENCES request.requests(id),
    change_id TEXT NOT NULL,
    branch TEXT NULL,
    status TEXT NOT NULL CHECK (status IN ('preparing','ready','archived','abandoned')),
    tasks_sync_state TEXT NOT NULL CHECK (tasks_sync_state IN ('in_sync','pending','failed')),
    tasks_sync_digest TEXT NULL,
    tasks_sync_error TEXT NULL,
    tasks_sync_next_retry_at TIMESTAMPTZ NULL,
    commit_sha TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    version BIGINT NOT NULL DEFAULT 1,
    UNIQUE (tenant_id, request_id)
);

CREATE INDEX idx_openspec_changes_sync ON request.openspec_changes (tenant_id, status, tasks_sync_state);

ALTER TABLE request.project_engine_settings ENABLE ROW LEVEL SECURITY;
ALTER TABLE request.project_engine_settings FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON request.project_engine_settings
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

ALTER TABLE request.openspec_changes ENABLE ROW LEVEL SECURITY;
ALTER TABLE request.openspec_changes FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON request.openspec_changes
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

ALTER TABLE request.requests ADD COLUMN solution_engine TEXT NULL CHECK (solution_engine IN ('native','openspec'));
ALTER TABLE request.analysis_runs ADD COLUMN engine TEXT NOT NULL DEFAULT 'native';
