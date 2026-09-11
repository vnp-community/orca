-- MySQL/TiDB variant of postgres/0003_template_parent_chain.up.sql.
ALTER TABLE templates ADD COLUMN parent_template_id CHAR(36) NULL,
    ADD CONSTRAINT fk_workflow_templates_parent FOREIGN KEY (parent_template_id) REFERENCES templates(id) ON DELETE SET NULL;

-- Walked by ResolveChain's recursive CTE — MySQL 8.0+/TiDB both support
-- WITH RECURSIVE, so internal/adapter/mysql.Repository.ResolveChain uses
-- the identical recursive-CTE shape as the Postgres adapter, no
-- application-side chain-walking fallback needed.
CREATE INDEX idx_workflow_templates_parent ON templates (parent_template_id);
