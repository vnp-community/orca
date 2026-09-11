-- MySQL translation of migrations/postgres/0010_execution_links.up.sql.
CREATE TABLE execution_links (
    id                CHAR(36) NOT NULL PRIMARY KEY DEFAULT (UUID()),
    tenant_id         CHAR(36) NOT NULL,
    task_id           CHAR(36) NOT NULL,
    engine            VARCHAR(20) NOT NULL,
    external_ref_id   TEXT NOT NULL, -- coordinator_run_id / workflow execution_id; empty for Engine 1
    status_mirror     VARCHAR(20) NOT NULL DEFAULT 'in_progress',
    started_at        TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    completed_at      TIMESTAMP(6) NULL,
    created_at        TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    CONSTRAINT execution_links_engine_check CHECK (engine IN ('direct_agent','orchestration','workflow')),
    CONSTRAINT fk_execution_links_task FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
) ENGINE=InnoDB;

-- MySQL 8.0 supports DESC key parts in an index definition (unlike 5.7,
-- which accepted but silently ignored DESC) — this genuinely produces a
-- descending index, matching Postgres's idx_execution_links_task exactly.
CREATE INDEX idx_execution_links_task ON execution_links (task_id, started_at DESC);

-- workflow_template_id is NOT added here — migrations/mysql/0009 already
-- added it, same non-collision note as the Postgres version.
ALTER TABLE tasks
  ADD COLUMN active_execution_link_id CHAR(36) NULL,
  ADD CONSTRAINT fk_tasks_active_execution_link FOREIGN KEY (active_execution_link_id) REFERENCES execution_links(id);
