-- Outcome Review (CR-OR-002): what a context run produced, kept so the next review does not ask the agent again. kind=design is
-- the locked design (reused by the delivered phase and by later runs); kind=context is a whole checked result, keyed by its digest.
CREATE TABLE request.outcome_review_contexts (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    request_id UUID NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('design','context')),
    digest TEXT NOT NULL,
    prompt_version TEXT NOT NULL DEFAULT '',
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    expires_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT uq_outcome_review_contexts UNIQUE (tenant_id, request_id, kind, digest)
);
CREATE INDEX idx_outcome_review_contexts_latest ON request.outcome_review_contexts (tenant_id, request_id, kind, created_at DESC);

ALTER TABLE request.outcome_review_contexts ENABLE ROW LEVEL SECURITY;
ALTER TABLE request.outcome_review_contexts FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON request.outcome_review_contexts
    AS PERMISSIVE FOR ALL TO PUBLIC
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);
