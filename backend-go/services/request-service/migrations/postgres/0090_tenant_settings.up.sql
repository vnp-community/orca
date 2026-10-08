-- request_flow_enabled defaults to false: a tenant must be switched on explicitly (CR-REQ-025).
CREATE TABLE request.tenant_settings (
    tenant_id UUID PRIMARY KEY,
    request_flow_enabled BOOLEAN NOT NULL DEFAULT false,
    updated_by TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE request.tenant_settings ENABLE ROW LEVEL SECURITY;
ALTER TABLE request.tenant_settings FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON request.tenant_settings FOR ALL
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);
