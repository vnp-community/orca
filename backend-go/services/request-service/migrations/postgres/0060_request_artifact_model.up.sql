-- CR-REQ-027: Request content columns, Solution artifact columns and four append-only artifact tables.
-- Content columns are written only by the append-revision use case after creation (see request_content_write_guard_test.go).
ALTER TABLE request.requests
    ADD COLUMN content_schema_version INT NOT NULL DEFAULT 1,
    ADD COLUMN acceptance_criteria JSONB NOT NULL DEFAULT '[]',
    ADD COLUMN type_fields JSONB NOT NULL DEFAULT '{}',
    ADD COLUMN content_revision INT NOT NULL DEFAULT 1,
    ADD COLUMN content_digest TEXT NOT NULL DEFAULT '';

-- seq stays nullable: NULLs never collide in the unique index, and a solution writer that has not
-- minted a seq yet must not fail its INSERT. MintSolutionID assigns it; backfill numbers existing rows.
ALTER TABLE request.solutions
    ADD COLUMN seq INT NULL,
    ADD COLUMN schema_version INT NOT NULL DEFAULT 1,
    ADD COLUMN provenance JSONB NOT NULL DEFAULT '{}',
    ADD COLUMN input_request_revision INT NOT NULL DEFAULT 1,
    ADD COLUMN content_digest TEXT NOT NULL DEFAULT '';

-- The owner is subject to FORCE RLS, so lift it for the backfill statement only.
ALTER TABLE request.solutions NO FORCE ROW LEVEL SECURITY;
UPDATE request.solutions s SET seq = n.rn
FROM (SELECT id, ROW_NUMBER() OVER (PARTITION BY tenant_id, request_id ORDER BY created_at, id) AS rn FROM request.solutions) n
WHERE s.id = n.id;
ALTER TABLE request.solutions FORCE ROW LEVEL SECURITY;

ALTER TABLE request.solutions ADD CONSTRAINT solutions_request_seq UNIQUE (tenant_id, request_id, seq);

-- Append-only: nothing in the service issues UPDATE or DELETE against this table.
CREATE TABLE request.request_revisions (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    request_id UUID NOT NULL,
    revision INT NOT NULL CHECK (revision >= 1),
    cause TEXT NOT NULL CHECK (cause IN ('created','clarification_answered','edited','type_changed')),
    snapshot JSONB NOT NULL,
    digest TEXT NOT NULL,
    actor_id TEXT NULL,
    actor_kind TEXT NOT NULL CHECK (actor_kind IN ('ai','user','system')),
    clarification_id UUID NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT request_revisions_unique_revision UNIQUE (tenant_id, request_id, revision)
);

CREATE TABLE request.artifact_index (
    tenant_id UUID NOT NULL,
    display_id TEXT NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('request','solution','option','plan','phase','task')),
    request_id UUID NOT NULL,
    artifact_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, display_id)
);
CREATE INDEX idx_artifact_index_artifact ON request.artifact_index (tenant_id, artifact_id);
CREATE INDEX idx_artifact_index_request ON request.artifact_index (tenant_id, request_id);

-- contains/depends_on live in task-service and spawned_by in request_links; they are not copied here.
CREATE TABLE request.artifact_relations (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    request_id UUID NOT NULL,
    rel TEXT NOT NULL CHECK (rel IN ('derived_from','implements','supersedes','evidenced_by','verifies')),
    from_kind TEXT NOT NULL,
    from_id TEXT NOT NULL,
    to_kind TEXT NOT NULL,
    to_id TEXT NOT NULL,
    created_by_run_id UUID NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT artifact_relations_unique_edge UNIQUE (tenant_id, rel, from_kind, from_id, to_kind, to_id)
);
CREATE INDEX idx_artifact_relations_request ON request.artifact_relations (tenant_id, request_id);

-- Derived data: replaced as a block inside the transaction that commits a Plan.
CREATE TABLE request.request_coverage (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    request_id UUID NOT NULL,
    plan_task_id UUID NOT NULL,
    ac_id TEXT NOT NULL,
    task_id UUID NOT NULL,
    check_id TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_request_coverage_plan ON request.request_coverage (tenant_id, request_id, plan_task_id);

DO $$
DECLARE
    t text;
BEGIN
    FOREACH t IN ARRAY ARRAY['request_revisions', 'artifact_index', 'artifact_relations', 'request_coverage'] LOOP
        EXECUTE format('ALTER TABLE request.%I ENABLE ROW LEVEL SECURITY', t);
        EXECUTE format('ALTER TABLE request.%I FORCE ROW LEVEL SECURITY', t);
        EXECUTE format('DROP POLICY IF EXISTS tenant_isolation ON request.%I', t);
        EXECUTE format('CREATE POLICY tenant_isolation ON request.%I FOR ALL USING (tenant_id = NULLIF(current_setting(''app.tenant_id'', true), '''')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting(''app.tenant_id'', true), '''')::uuid)', t);
    END LOOP;
END
$$;
