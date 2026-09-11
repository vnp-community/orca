-- MySQL translation of migrations/postgres/0011_task_widened_fields.up.sql.
--
-- labels: Postgres TEXT[] has no MySQL array type. A JSON array is the
-- closest structural equivalent (internal/adapter/mysql marshals/unmarshals
-- domain.Task.Labels ([]string) to/from this column, mirroring how
-- ai_plan_json already round-trips JSON text elsewhere in this schema).
-- MySQL 8.0.13+ allows an expression DEFAULT for JSON columns
-- (`DEFAULT (expr)`), which is what makes `DEFAULT (JSON_ARRAY())` valid
-- here — plain literal defaults are still not accepted for JSON/TEXT/BLOB.
--
-- workflow_exec_id: Postgres is TEXT NOT NULL DEFAULT ''. MySQL cannot
-- default a bare TEXT column to a literal '' without the same
-- expression-DEFAULT syntax (`DEFAULT ('')`) — used narrow VARCHAR(255)
-- instead since this column always holds a short execution-tracking id
-- (mirrors execution_links.external_ref_id's shape), which also sidesteps
-- needing the expression-default form at all.
ALTER TABLE tasks
  ADD COLUMN labels           JSON NOT NULL DEFAULT (JSON_ARRAY()),
  ADD COLUMN reporter_id      CHAR(36) NULL,   -- logical FK -> tenant-service
  ADD COLUMN workflow_exec_id VARCHAR(255) NOT NULL DEFAULT '',
  ADD COLUMN done_subtasks    INT NOT NULL DEFAULT 0,
  ADD COLUMN total_subtasks   INT NOT NULL DEFAULT 0;

-- share_token: VARCHAR(255) UNIQUE, nullable — MySQL's UNIQUE key permits
-- unlimited NULLs (same as Postgres), so "unset until GenerateShareLink
-- mints one" behaves identically. No separate idx_tasks_share_token index
-- is created here (unlike the Postgres migration's redundant partial
-- index) — the UNIQUE constraint above already provides an index MySQL
-- uses for GetByShareToken's `WHERE share_token = ?` equality lookup, so a
-- second index would only duplicate it, not add anything a partial index
-- would have that a full one doesn't already cover for an equality query.
ALTER TABLE tasks ADD COLUMN share_token VARCHAR(255) NULL;
ALTER TABLE tasks ADD CONSTRAINT tasks_share_token_unique UNIQUE (share_token);
