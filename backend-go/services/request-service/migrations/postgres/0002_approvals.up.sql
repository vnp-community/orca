CREATE TABLE request.approvals (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    request_id UUID NOT NULL REFERENCES request.requests(id),
    subject_type TEXT NOT NULL CHECK (subject_type IN ('request_type','solution','findings','answer','plan','phase','task_list','pre_deploy')),
    subject_id TEXT NOT NULL,
    stage TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('pending','approved','rejected','cancelled','expired')),
    requested_by TEXT NOT NULL,
    decided_by TEXT NULL,
    decided_at TIMESTAMPTZ NULL,
    comment TEXT NOT NULL DEFAULT '',
    due_at TIMESTAMPTZ NULL,
    version BIGINT NOT NULL DEFAULT 1,
    subject_digest TEXT NOT NULL,
    self_approval_allowed BOOLEAN NOT NULL DEFAULT TRUE,
    idempotency_key TEXT NULL,
    reminded_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX approvals_one_pending ON request.approvals (tenant_id, subject_type, subject_id) WHERE status='pending';
CREATE UNIQUE INDEX approvals_idem ON request.approvals (tenant_id, idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE INDEX approvals_by_request ON request.approvals (tenant_id, request_id, created_at);
CREATE INDEX approvals_due ON request.approvals (tenant_id, status, due_at);
CREATE INDEX approvals_inbox ON request.approvals (tenant_id, status, created_at);

ALTER TABLE request.approvals ENABLE ROW LEVEL SECURITY;
ALTER TABLE request.approvals FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON request.approvals
    AS PERMISSIVE FOR ALL
    TO PUBLIC
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);
