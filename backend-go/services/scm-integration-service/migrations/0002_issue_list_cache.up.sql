CREATE TABLE scm.issue_list_cache (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL,
    provider        TEXT NOT NULL,
    repo            TEXT NOT NULL,
    filter_hash     TEXT NOT NULL,      -- sha256 of the normalized IssueFilter
    issues_json     JSONB NOT NULL,
    cached_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at      TIMESTAMPTZ NOT NULL,
    UNIQUE (tenant_id, provider, repo, filter_hash)
);
CREATE INDEX idx_issue_list_cache_expires ON scm.issue_list_cache (expires_at);

ALTER TABLE scm.issue_list_cache ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON scm.issue_list_cache
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
