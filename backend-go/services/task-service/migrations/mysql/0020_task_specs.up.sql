-- MySQL translation of postgres/0020_task_specs.up.sql (no RLS: tenant_id filters in the repository).
CREATE TABLE task_specs (
    task_id        CHAR(36) NOT NULL PRIMARY KEY,
    tenant_id      CHAR(36) NOT NULL,
    schema_version INT NOT NULL,
    spec           JSON NOT NULL,
    digest         CHAR(64) NOT NULL,
    locked_at      TIMESTAMP(6) NULL,
    created_at     TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at     TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    version        BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT task_specs_schema_version_check CHECK (schema_version >= 1),
    CONSTRAINT fk_task_specs_task FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE,
    KEY idx_task_specs_tenant (tenant_id, locked_at)
) ENGINE=InnoDB;
