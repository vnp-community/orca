CREATE TABLE request.approval_policies (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    project_id UUID NULL,
    subject_type TEXT NOT NULL CHECK (subject_type IN ('request_type','solution','findings','answer','plan','phase','task_list','pre_deploy')),
    request_type TEXT NULL CHECK (request_type IN ('task','feature','bug','incident','problem','change','epic','story','subtask','release','other')),
    size TEXT NULL CHECK (size IN ('S','M','L')),
    urgency TEXT NULL CHECK (urgency IN ('normal','urgent')),
    approvers JSONB NOT NULL,
    allow_requester_approve BOOLEAN NOT NULL DEFAULT FALSE,
    due_after_seconds INT NULL,
    priority INT NOT NULL DEFAULT 0,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    version BIGINT NOT NULL DEFAULT 1,
    created_by TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX approval_policies_subject ON request.approval_policies (tenant_id, subject_type, enabled);

ALTER TABLE request.approval_policies ENABLE ROW LEVEL SECURITY;
ALTER TABLE request.approval_policies FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_policies ON request.approval_policies
    AS PERMISSIVE FOR ALL
    TO PUBLIC
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);


CREATE TABLE request.approval_approvers (
    approval_id UUID NOT NULL REFERENCES request.approvals(id),
    principal_kind TEXT NOT NULL CHECK (principal_kind IN ('user','team','role','reporter')),
    principal_id TEXT NOT NULL,
    tenant_id UUID NOT NULL,
    PRIMARY KEY (approval_id, principal_kind, principal_id)
);

CREATE INDEX approval_approvers_principal ON request.approval_approvers (tenant_id, principal_kind, principal_id);

ALTER TABLE request.approval_approvers ENABLE ROW LEVEL SECURITY;
ALTER TABLE request.approval_approvers FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_approvers ON request.approval_approvers
    AS PERMISSIVE FOR ALL
    TO PUBLIC
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);
