CREATE TABLE request.request_counters (
    tenant_id UUID PRIMARY KEY,
    next_number BIGINT NOT NULL
);

CREATE TABLE request.requests (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    project_id UUID,
    number BIGINT NOT NULL,
    title TEXT NOT NULL,
    body TEXT NOT NULL DEFAULT '',
    source_provider TEXT NOT NULL DEFAULT '',
    source_ref TEXT NOT NULL DEFAULT '',
    source_url TEXT NOT NULL DEFAULT '',
    source_site TEXT NOT NULL DEFAULT '',
    type TEXT CHECK (type IN ('change_request','bug','hotfix','task','spike','question','refactor','security','performance','docs','ops_request')),
    type_source TEXT CHECK (type_source IN ('ai','human')),
    size TEXT CHECK (size IN ('S','M','L')),
    urgency TEXT NOT NULL DEFAULT 'normal' CHECK (urgency IN ('normal','urgent')),
    confidence NUMERIC(4,3) CHECK (confidence >= 0 AND confidence <= 1),
    classification_reason TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'new' CHECK (status IN ('new','classifying','awaiting_type_confirmation','analyzing','awaiting_analysis_approval','planning','awaiting_plan_approval','executing','completed','request_backlog','cancelled')),
    returned_from_stage TEXT CHECK (returned_from_stage IN ('classification','analysis','plan','phase','task')),
    return_reason TEXT NOT NULL DEFAULT '',
    plan_task_id UUID,
    reporter_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    version BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT requests_backlog_stage CHECK ((status = 'request_backlog') = (returned_from_stage IS NOT NULL)),
    UNIQUE (tenant_id, number)
);

CREATE INDEX idx_requests_status_updated ON request.requests (tenant_id, status, updated_at DESC);
CREATE INDEX idx_requests_project_status ON request.requests (tenant_id, project_id, status);
CREATE INDEX idx_requests_plan_task ON request.requests (tenant_id, plan_task_id);
CREATE INDEX idx_requests_source ON request.requests (tenant_id, source_provider, source_site, source_ref);
CREATE INDEX idx_requests_pagination ON request.requests (tenant_id, created_at DESC, id DESC);

CREATE TABLE request.request_type_history (
    tenant_id UUID NOT NULL,
    request_id UUID NOT NULL,
    at TIMESTAMPTZ NOT NULL DEFAULT now(),
    from_type TEXT,
    to_type TEXT NOT NULL,
    actor_id UUID NOT NULL,
    actor_kind TEXT CHECK (actor_kind IN ('user','agent','system'))
);

CREATE INDEX idx_request_type_history_at ON request.request_type_history (request_id, at);

CREATE TABLE request.solutions (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    request_id UUID NOT NULL,
    options JSONB NOT NULL,
    chosen_option INT,
    version BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_solutions_created ON request.solutions (tenant_id, request_id, created_at);

CREATE TABLE request.request_links (
    tenant_id UUID NOT NULL,
    parent_request_id UUID NOT NULL,
    child_request_id UUID NOT NULL,
    reason TEXT CHECK (reason IN ('relates_to', 'blocks', 'is_blocked_by', 'duplicates')),
    PRIMARY KEY (tenant_id, parent_request_id, child_request_id),
    CHECK (parent_request_id <> child_request_id)
);

CREATE INDEX idx_request_links_child ON request.request_links (tenant_id, child_request_id);

CREATE TABLE request.request_idempotency (
    tenant_id UUID NOT NULL,
    source_provider TEXT NOT NULL,
    source_site TEXT NOT NULL DEFAULT '',
    source_ref TEXT NOT NULL CHECK (source_ref <> ''),
    request_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, source_provider, source_site, source_ref)
);

DO $$
DECLARE
    t text;
BEGIN
    FOREACH t IN ARRAY ARRAY['request_counters', 'requests', 'request_type_history', 'solutions', 'request_links', 'request_idempotency'] LOOP
        EXECUTE format('ALTER TABLE request.%I ENABLE ROW LEVEL SECURITY', t);
        EXECUTE format('ALTER TABLE request.%I FORCE ROW LEVEL SECURITY', t);
        EXECUTE format('DROP POLICY IF EXISTS tenant_isolation ON request.%I', t);
        EXECUTE format('CREATE POLICY tenant_isolation ON request.%I FOR ALL USING (tenant_id = NULLIF(current_setting(''app.tenant_id'', true), '''')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting(''app.tenant_id'', true), '''')::uuid)', t);
    END LOOP;
END
$$;
