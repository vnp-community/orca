-- MySQL translation of migrations/postgres/0012_task_sources.up.sql.
--
-- project_key: MySQL unique indexes treat NULLs as distinct, so a nullable
-- project_id would let two sourceless-project rows collide silently. A
-- generated column that folds NULL to '' gives the unique key the same
-- "NULL project counts as one project" meaning the Postgres COALESCE does.
CREATE TABLE task_sources (
    task_id     CHAR(36) NOT NULL PRIMARY KEY,
    tenant_id   CHAR(36) NOT NULL,
    project_id  CHAR(36) NULL,
    project_key CHAR(36) GENERATED ALWAYS AS (COALESCE(project_id, '')) STORED,
    provider    VARCHAR(20) NOT NULL,
    ref         VARCHAR(255) NOT NULL,
    url         TEXT NOT NULL,
    created_at  TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    CONSTRAINT task_sources_provider_check CHECK (provider IN ('jira','linear','github','gitlab')),
    CONSTRAINT fk_task_sources_task FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE,
    UNIQUE KEY idx_task_sources_unique (tenant_id, project_key, provider, ref)
) ENGINE=InnoDB;
