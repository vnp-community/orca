-- Outcome Review (CR-OR-011): the drawings of a review. A drawing is keyed by the digest of its diagram model and of the drawing
-- preset, not by the review or the commit, so it is paid for once and reused wherever the same model appears. The HTML is a blob
-- stored once per Request and digest; content is TEXT so erasing a Request clears it like any other free text.
CREATE TABLE request.review_artifact_blobs (
    tenant_id UUID NOT NULL,
    request_id UUID NOT NULL,
    digest TEXT NOT NULL,
    media_type TEXT NOT NULL,
    size_bytes INT NOT NULL CHECK (size_bytes >= 0),
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (tenant_id, request_id, digest)
);

-- A failed attempt keeps a row (no blob) so the review shows why a drawing is missing and does not retry by itself.
-- active_key is set only on the one active drawing of a slot, so a second active one cannot be inserted.
CREATE TABLE request.review_artifacts (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    request_id UUID NOT NULL,
    section TEXT NOT NULL CHECK (section IN ('architecture','flow','behavior','data')),
    variant TEXT NOT NULL CHECK (variant IN ('design','delta')),
    name TEXT NOT NULL DEFAULT '',
    diagram_type TEXT NOT NULL,
    input_digest TEXT NOT NULL,
    preset_digest TEXT NOT NULL,
    skill_version TEXT NOT NULL,
    revision INT NOT NULL CHECK (revision >= 1),
    state TEXT NOT NULL CHECK (state IN ('active','superseded','failed')),
    origin TEXT NOT NULL CHECK (origin IN ('auto','regenerated')),
    reason TEXT NOT NULL DEFAULT '',
    error_code TEXT NOT NULL DEFAULT '',
    blob_digest TEXT NOT NULL DEFAULT '',
    size_bytes INT NOT NULL DEFAULT 0,
    view_box TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    self_check TEXT NOT NULL DEFAULT '',
    model JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_by UUID NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    active_key TEXT NULL,
    CONSTRAINT uq_review_artifacts_revision UNIQUE (tenant_id, request_id, section, variant, name, input_digest, preset_digest, skill_version, revision),
    CONSTRAINT uq_review_artifacts_active UNIQUE (active_key)
);
CREATE INDEX idx_review_artifacts_slot ON request.review_artifacts (tenant_id, request_id, section, variant, name, created_at DESC);
CREATE INDEX idx_review_artifacts_age ON request.review_artifacts (tenant_id, state, created_at);

ALTER TABLE request.review_artifact_blobs ENABLE ROW LEVEL SECURITY;
ALTER TABLE request.review_artifact_blobs FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON request.review_artifact_blobs
    AS PERMISSIVE FOR ALL TO PUBLIC
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

ALTER TABLE request.review_artifacts ENABLE ROW LEVEL SECURITY;
ALTER TABLE request.review_artifacts FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON request.review_artifacts
    AS PERMISSIVE FOR ALL TO PUBLIC
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);
