-- Outcome Review (CR-OR-011): the drawings of a review. A drawing is keyed by the digest of its diagram model and of the drawing
-- preset, not by the review or the commit, so it is paid for once and reused wherever the same model appears. The HTML is a blob
-- stored once per Request and digest; content is text so erasing a Request clears it like any other free text.
CREATE TABLE review_artifact_blobs (
    tenant_id CHAR(36) NOT NULL,
    request_id CHAR(36) NOT NULL,
    digest VARCHAR(64) NOT NULL,
    media_type VARCHAR(40) NOT NULL,
    size_bytes INT NOT NULL CHECK (size_bytes >= 0),
    content MEDIUMTEXT NOT NULL,
    created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    PRIMARY KEY (tenant_id, request_id, digest)
) ENGINE=InnoDB;

-- A failed attempt keeps a row (no blob) so the review shows why a drawing is missing and does not retry by itself.
-- active_key is set only on the one active drawing of a slot (NULLs do not collide), so a second active one cannot be inserted.
CREATE TABLE review_artifacts (
    id CHAR(36) PRIMARY KEY,
    tenant_id CHAR(36) NOT NULL,
    request_id CHAR(36) NOT NULL,
    section VARCHAR(16) NOT NULL CHECK (section IN ('architecture','flow','behavior','data')),
    variant VARCHAR(8) NOT NULL CHECK (variant IN ('design','delta')),
    name VARCHAR(60) NOT NULL DEFAULT '',
    diagram_type VARCHAR(24) NOT NULL,
    input_digest VARCHAR(64) NOT NULL,
    preset_digest VARCHAR(64) NOT NULL,
    skill_version VARCHAR(64) NOT NULL,
    revision INT NOT NULL CHECK (revision >= 1),
    state VARCHAR(12) NOT NULL CHECK (state IN ('active','superseded','failed')),
    origin VARCHAR(12) NOT NULL CHECK (origin IN ('auto','regenerated')),
    reason VARCHAR(200) NOT NULL DEFAULT '',
    error_code VARCHAR(80) NOT NULL DEFAULT '',
    blob_digest VARCHAR(64) NOT NULL DEFAULT '',
    size_bytes INT NOT NULL DEFAULT 0,
    view_box VARCHAR(60) NOT NULL DEFAULT '',
    title VARCHAR(400) NOT NULL DEFAULT '',
    description VARCHAR(1000) NOT NULL DEFAULT '',
    self_check VARCHAR(400) NOT NULL DEFAULT '',
    model JSON NOT NULL,
    created_by CHAR(36) NULL,
    created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    active_key VARCHAR(200) NULL,
    UNIQUE KEY uq_review_artifacts_revision (tenant_id, request_id, section, variant, name, input_digest, preset_digest, skill_version, revision),
    UNIQUE KEY uq_review_artifacts_active (active_key),
    INDEX idx_review_artifacts_slot (tenant_id, request_id, section, variant, name, created_at DESC),
    INDEX idx_review_artifacts_age (tenant_id, state, created_at)
) ENGINE=InnoDB;
