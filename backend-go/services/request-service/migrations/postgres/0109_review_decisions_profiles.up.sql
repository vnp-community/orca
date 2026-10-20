-- Outcome Review (CR-OR-022, CR-OR-012, CR-OR-013): what a person decided about a review, who approved it, how a regeneration is
-- tracked, and the drawing profile a tenant or project saved.

-- One decision in force for each (review, item). A changed mind is a new row; the old one is kept as history with active_key cleared.
CREATE TABLE request.outcome_review_decisions (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    review_id UUID NOT NULL,
    request_id UUID NOT NULL,
    item_key TEXT NOT NULL,
    decision TEXT NOT NULL CHECK (decision IN ('accept_exception','send_back','dismiss')),
    note TEXT NOT NULL DEFAULT '',
    child_request_id UUID NULL,
    decided_by UUID NOT NULL,
    decided_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    active_key TEXT NULL,
    CONSTRAINT uq_outcome_review_decisions_active UNIQUE (active_key)
);
CREATE INDEX idx_outcome_review_decisions_review ON request.outcome_review_decisions (tenant_id, review_id, decided_at);

ALTER TABLE request.outcome_reviews
    ADD COLUMN approved_by UUID NULL,
    ADD COLUMN approved_at TIMESTAMPTZ NULL,
    ADD COLUMN approval_note TEXT NOT NULL DEFAULT '';

-- reason is why a person regenerated (empty for a run the system started); client_key makes a repeated request one run.
ALTER TABLE request.outcome_review_runs
    ADD COLUMN reason TEXT NOT NULL DEFAULT '',
    ADD COLUMN client_key TEXT NULL,
    ADD COLUMN created_by UUID NULL;
CREATE UNIQUE INDEX uq_outcome_review_runs_client ON request.outcome_review_runs (tenant_id, request_id, client_key);
CREATE INDEX idx_outcome_review_runs_reason ON request.outcome_review_runs (tenant_id, request_id, started_at) WHERE reason <> '';

-- project_key is the project id, or empty for the tenant layer.
CREATE TABLE request.review_drawing_profiles (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    project_key TEXT NOT NULL DEFAULT '',
    spec JSONB NOT NULL,
    spec_digest TEXT NOT NULL,
    version BIGINT NOT NULL DEFAULT 1,
    updated_by UUID NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    CONSTRAINT uq_review_drawing_profiles_scope UNIQUE (tenant_id, project_key)
);

ALTER TABLE request.outcome_review_decisions ENABLE ROW LEVEL SECURITY;
ALTER TABLE request.outcome_review_decisions FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON request.outcome_review_decisions
    AS PERMISSIVE FOR ALL TO PUBLIC
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

ALTER TABLE request.review_drawing_profiles ENABLE ROW LEVEL SECURITY;
ALTER TABLE request.review_drawing_profiles FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON request.review_drawing_profiles
    AS PERMISSIVE FOR ALL TO PUBLIC
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);
