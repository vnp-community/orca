-- Outcome Review (CR-OR-001): one row per (request, phase, head). model is the whole review as JSON; the other columns are what the
-- service looks reviews up by. Rows are never deleted by the service; an old review is marked superseded.
CREATE TABLE request.outcome_reviews (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    request_id UUID NOT NULL,
    phase TEXT NOT NULL CHECK (phase IN ('design','delivered')),
    head_key TEXT NOT NULL,
    head_commit TEXT NOT NULL DEFAULT '',
    base_commit TEXT NOT NULL DEFAULT '',
    inputs_digest TEXT NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('draft','ready','stale','approved','superseded')),
    model JSONB NOT NULL,
    kg_snapshot TEXT NOT NULL DEFAULT '',
    design_digest TEXT NOT NULL DEFAULT '',
    context_digest TEXT NOT NULL DEFAULT '',
    created_by UUID NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    version BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_outcome_reviews_key UNIQUE (tenant_id, request_id, phase, head_key)
);
CREATE INDEX idx_outcome_reviews_request ON request.outcome_reviews (tenant_id, request_id, created_at DESC, id DESC);

-- One row per agent run of a review. active_key is set only while the run is queued or running, so a second active run of the
-- same kind for a Request cannot be inserted; it is cleared when the run ends.
CREATE TABLE request.outcome_review_runs (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    review_id UUID NOT NULL,
    request_id UUID NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('context','draw')),
    status TEXT NOT NULL CHECK (status IN ('queued','running','succeeded','failed','cancelled')),
    scope TEXT NOT NULL DEFAULT '',
    agent_run_ref TEXT NOT NULL DEFAULT '',
    cost_estimate TEXT NOT NULL DEFAULT '',
    cost_actual TEXT NOT NULL DEFAULT '',
    error_code TEXT NOT NULL DEFAULT '',
    active_key TEXT NULL,
    started_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    finished_at TIMESTAMPTZ NULL,
    CONSTRAINT uq_outcome_review_runs_active UNIQUE (active_key)
);
CREATE INDEX idx_outcome_review_runs_review ON request.outcome_review_runs (tenant_id, review_id, started_at DESC);
CREATE INDEX idx_outcome_review_runs_request ON request.outcome_review_runs (tenant_id, request_id, kind, started_at DESC);

ALTER TABLE request.outcome_reviews ENABLE ROW LEVEL SECURITY;
ALTER TABLE request.outcome_reviews FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON request.outcome_reviews
    AS PERMISSIVE FOR ALL TO PUBLIC
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

ALTER TABLE request.outcome_review_runs ENABLE ROW LEVEL SECURITY;
ALTER TABLE request.outcome_review_runs FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON request.outcome_review_runs
    AS PERMISSIVE FOR ALL TO PUBLIC
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);
