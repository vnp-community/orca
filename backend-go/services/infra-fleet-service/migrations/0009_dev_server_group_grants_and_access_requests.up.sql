-- CR-DS-007 (docs/crs/v2/dev-server/CR-DS-007-department-based-access-control.md)
-- and CR-DS-008 (docs/crs/v2/dev-server/CR-DS-008-first-login-department-gate-and-access-request.md).

CREATE TABLE infra.dev_server_group_grants (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id             UUID NOT NULL,
    dev_server_group_id   UUID NOT NULL REFERENCES infra.dev_server_groups(id),
    grantee_kind          TEXT NOT NULL CHECK (grantee_kind IN ('department', 'team')),
    -- grantee_id is a logical FK into tenant-service's departments/teams —
    -- a different service's database, so no physical FK (see
    -- CR-DS-007 §2's "resolve at the edge" note).
    grantee_id            TEXT NOT NULL,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (dev_server_group_id, grantee_kind, grantee_id)
);

CREATE INDEX idx_infra_dev_server_group_grants_tenant ON infra.dev_server_group_grants (tenant_id);
CREATE INDEX idx_infra_dev_server_group_grants_group ON infra.dev_server_group_grants (dev_server_group_id);

ALTER TABLE infra.dev_server_group_grants ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON infra.dev_server_group_grants
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);

CREATE TABLE infra.dev_server_access_requests (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id             UUID NOT NULL,
    user_id               UUID NOT NULL,
    dev_server_group_id   UUID NOT NULL REFERENCES infra.dev_server_groups(id),
    status                TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'approved', 'rejected')),
    message               TEXT NOT NULL DEFAULT '',
    grantee_kind          TEXT NOT NULL CHECK (grantee_kind IN ('department', 'team')),
    grantee_id            TEXT NOT NULL,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at           TIMESTAMPTZ,
    resolved_by           UUID
);

CREATE INDEX idx_infra_dev_server_access_requests_tenant ON infra.dev_server_access_requests (tenant_id, status);

ALTER TABLE infra.dev_server_access_requests ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON infra.dev_server_access_requests
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
