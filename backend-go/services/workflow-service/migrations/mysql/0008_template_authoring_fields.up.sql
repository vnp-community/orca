-- MySQL/TiDB variant of postgres/0008_template_authoring_fields.up.sql.
--
-- tags: Postgres uses TEXT[] + a GIN index for the `tags @> $filter`
-- AND-containment query. MySQL/TiDB has no array type or GIN-equivalent
-- index — tags is stored as a JSON array instead, and
-- internal/adapter/mysql.Repository.ListTemplates builds one
-- `JSON_CONTAINS(tags, ?, '$')` predicate per requested tag (AND'd
-- together), matching the Postgres containment semantics exactly (every
-- listed tag must be present) but as a full table/index-prefix scan
-- rather than a GIN-accelerated lookup — a real, deliberate cost of this
-- dialect, called out here rather than silently accepted.
--
-- overrides/inject_steps/remove_steps: JSONB -> JSON, same as dag_json in
-- 0001_init — these columns are read/written whole by Go (json.Unmarshal
-- into a Go type), never queried by a JSONB operator, so nothing
-- functional is lost.
ALTER TABLE templates
  ADD COLUMN description TEXT NULL,
  ADD COLUMN tags JSON NOT NULL DEFAULT (JSON_ARRAY()),
  ADD COLUMN owner_id CHAR(36) NULL,             -- backfilled below, then NOT NULL in a follow-up migration (see postgres variant's identical comment — no such follow-up exists yet in either dialect)
  ADD COLUMN usage_count INT NOT NULL DEFAULT 0,
  ADD COLUMN overrides JSON NOT NULL DEFAULT (JSON_OBJECT()),
  ADD COLUMN inject_steps JSON NOT NULL DEFAULT (JSON_ARRAY()),
  ADD COLUMN remove_steps JSON NOT NULL DEFAULT (JSON_ARRAY()),
  ADD COLUMN cloned_from_template_id CHAR(36) NULL;

ALTER TABLE templates
  ADD CONSTRAINT fk_workflow_templates_cloned_from FOREIGN KEY (cloned_from_template_id) REFERENCES templates(id) ON DELETE SET NULL;

-- Backfill: same reasoning as the Postgres variant — no owner history
-- exists for pre-migration rows, so each row's own tenant is the only
-- safe placeholder.
UPDATE templates SET owner_id = tenant_id WHERE owner_id IS NULL;

CREATE INDEX idx_workflow_templates_owner ON templates(tenant_id, owner_id);
