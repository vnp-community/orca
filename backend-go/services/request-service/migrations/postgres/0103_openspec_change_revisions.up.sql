-- One row per OpenSpec change a Plan run produced (CR-REQ-045), so the versions of a Plan stay listable after a revision
-- rewrites request.openspec_changes. The index (paths, sizes, hashes) is kept for every version; the full text only for the
-- newest ones, and bundle_json is NULL once a version falls out of that window.
CREATE TABLE request.openspec_change_revisions (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    request_id UUID NOT NULL REFERENCES request.requests(id),
    revision INT NOT NULL CHECK (revision > 0),
    run_id UUID NOT NULL,
    commit_sha TEXT NOT NULL DEFAULT '',
    tasks_md_digest TEXT NOT NULL DEFAULT '',
    bundle_digest TEXT NOT NULL,
    files_json JSONB NOT NULL,
    bundle_json JSONB NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, request_id, revision),
    UNIQUE (tenant_id, run_id)
);

CREATE INDEX openspec_change_revisions_by_request ON request.openspec_change_revisions (tenant_id, request_id, revision DESC);

ALTER TABLE request.openspec_change_revisions ENABLE ROW LEVEL SECURITY;
ALTER TABLE request.openspec_change_revisions FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON request.openspec_change_revisions
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);
