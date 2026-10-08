-- CR-REQ-027, MySQL 8.0.16+. JSON columns use expression defaults (8.0.13+) so older INSERTs that
-- do not know these columns keep working; every query below this layer filters by tenant_id.
ALTER TABLE requests
    ADD COLUMN content_schema_version INT NOT NULL DEFAULT 1,
    ADD COLUMN acceptance_criteria JSON NOT NULL DEFAULT (JSON_ARRAY()),
    ADD COLUMN type_fields JSON NOT NULL DEFAULT (JSON_OBJECT()),
    ADD COLUMN content_revision INT NOT NULL DEFAULT 1,
    ADD COLUMN content_digest CHAR(64) NOT NULL DEFAULT '';

-- seq stays nullable: NULLs never collide in the unique key (see the Postgres migration).
ALTER TABLE solutions
    ADD COLUMN seq INT NULL,
    ADD COLUMN schema_version INT NOT NULL DEFAULT 1,
    ADD COLUMN provenance JSON NOT NULL DEFAULT (JSON_OBJECT()),
    ADD COLUMN input_request_revision INT NOT NULL DEFAULT 1,
    ADD COLUMN content_digest CHAR(64) NOT NULL DEFAULT '';

UPDATE solutions s
JOIN (SELECT id, ROW_NUMBER() OVER (PARTITION BY tenant_id, request_id ORDER BY created_at, id) AS rn FROM solutions) n ON n.id = s.id
SET s.seq = n.rn;

ALTER TABLE solutions ADD CONSTRAINT solutions_request_seq UNIQUE (tenant_id, request_id, seq);

-- Append-only: nothing in the service issues UPDATE or DELETE against this table.
CREATE TABLE request_revisions (
    id CHAR(36) PRIMARY KEY,
    tenant_id CHAR(36) NOT NULL,
    request_id CHAR(36) NOT NULL,
    revision INT NOT NULL CHECK (revision >= 1),
    cause VARCHAR(32) NOT NULL CHECK (cause IN ('created','clarification_answered','edited','type_changed')),
    snapshot JSON NOT NULL,
    digest CHAR(64) NOT NULL,
    actor_id VARCHAR(64) NULL,
    actor_kind VARCHAR(16) NOT NULL CHECK (actor_kind IN ('ai','user','system')),
    clarification_id CHAR(36) NULL,
    created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    UNIQUE KEY request_revisions_unique_revision (tenant_id, request_id, revision)
) ENGINE=InnoDB;

CREATE TABLE artifact_index (
    tenant_id CHAR(36) NOT NULL,
    display_id VARCHAR(80) NOT NULL,
    kind VARCHAR(16) NOT NULL CHECK (kind IN ('request','solution','option','plan','phase','task')),
    request_id CHAR(36) NOT NULL,
    artifact_id CHAR(36) NOT NULL,
    created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    PRIMARY KEY (tenant_id, display_id),
    INDEX idx_artifact_index_artifact (tenant_id, artifact_id),
    INDEX idx_artifact_index_request (tenant_id, request_id)
) ENGINE=InnoDB;

-- Unique key width: 36 + 20 + 16 + 160 + 16 + 160 characters x 4 bytes (utf8mb4) = 1584 bytes, under the 3072 limit.
CREATE TABLE artifact_relations (
    id CHAR(36) PRIMARY KEY,
    tenant_id CHAR(36) NOT NULL,
    request_id CHAR(36) NOT NULL,
    rel VARCHAR(20) NOT NULL CHECK (rel IN ('derived_from','implements','supersedes','evidenced_by','verifies')),
    from_kind VARCHAR(16) NOT NULL,
    from_id VARCHAR(160) NOT NULL,
    to_kind VARCHAR(16) NOT NULL,
    to_id VARCHAR(160) NOT NULL,
    created_by_run_id CHAR(36) NULL,
    created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    UNIQUE KEY artifact_relations_unique_edge (tenant_id, rel, from_kind, from_id, to_kind, to_id),
    INDEX idx_artifact_relations_request (tenant_id, request_id)
) ENGINE=InnoDB;

CREATE TABLE request_coverage (
    id CHAR(36) PRIMARY KEY,
    tenant_id CHAR(36) NOT NULL,
    request_id CHAR(36) NOT NULL,
    plan_task_id CHAR(36) NOT NULL,
    ac_id VARCHAR(16) NOT NULL,
    task_id CHAR(36) NOT NULL,
    check_id VARCHAR(64) NULL,
    created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    INDEX idx_request_coverage_plan (tenant_id, request_id, plan_task_id)
) ENGINE=InnoDB;
