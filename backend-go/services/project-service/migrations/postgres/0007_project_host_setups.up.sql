-- project.project_host_setups — the pre-project wizard record
-- projectHostSetup.* manages: name a dev server + an existing folder path
-- on it, validate, then finalize into a real Project + Repo. dev_server_id
-- is a logical FK -> infra-fleet-service.dev_servers, validated via gRPC
-- at create/setupExistingFolder time, never joined in SQL
-- (05-data-architecture.md's "no cross-database FK" rule).
CREATE TABLE project.project_host_setups (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     UUID NOT NULL,
    dev_server_id UUID NOT NULL,
    folder_path   TEXT NOT NULL,
    display_name  TEXT,
    status        TEXT NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending', 'validated', 'completed', 'failed')),
    project_id    UUID REFERENCES project.projects (id) ON DELETE SET NULL,
    created_by    UUID NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_project_host_setups_tenant ON project.project_host_setups (tenant_id);

ALTER TABLE project.project_host_setups ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON project.project_host_setups
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
