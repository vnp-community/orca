ALTER TABLE request.requests ADD COLUMN returned_category TEXT
    CONSTRAINT requests_returned_category_values CHECK (returned_category IN ('missing_info','infeasible','blocked_dependency','rejected','other'));

-- Backfill before the pairing CHECK; the owner is subject to FORCE RLS, so lift it for this statement only.
ALTER TABLE request.requests NO FORCE ROW LEVEL SECURITY;
UPDATE request.requests SET returned_category = 'other' WHERE status = 'request_backlog';
ALTER TABLE request.requests FORCE ROW LEVEL SECURITY;

ALTER TABLE request.requests ADD CONSTRAINT requests_backlog_category
    CHECK ((status = 'request_backlog') = (returned_category IS NOT NULL));

CREATE TABLE request.request_return_history (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    request_id UUID NOT NULL,
    action TEXT NOT NULL CHECK (action IN ('returned','reopened','cancelled')),
    stage TEXT CHECK (stage IN ('classification','analysis','plan','phase','task')),
    category TEXT CHECK (category IN ('missing_info','infeasible','blocked_dependency','rejected','other')),
    reason TEXT NOT NULL DEFAULT '',
    actor_id UUID,
    actor_kind TEXT NOT NULL CHECK (actor_kind IN ('user','agent','system')),
    at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_request_return_history ON request.request_return_history (tenant_id, request_id, at);

ALTER TABLE request.request_return_history ENABLE ROW LEVEL SECURITY;
ALTER TABLE request.request_return_history FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON request.request_return_history;
CREATE POLICY tenant_isolation ON request.request_return_history FOR ALL
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);
