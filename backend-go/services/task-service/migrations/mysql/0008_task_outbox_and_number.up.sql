-- MySQL translation of migrations/postgres/0008_task_outbox_and_number.up.sql.
ALTER TABLE tasks ADD COLUMN task_number BIGINT NULL;
ALTER TABLE tasks ADD COLUMN pr_url TEXT;

-- MySQL/TiDB has no CREATE SEQUENCE (that's a Postgres/MariaDB-only
-- object) — task_number_seq emulates one via a dedicated AUTO_INCREMENT
-- table: internal/adapter/mysql.Repository.Create obtains the next value
-- by `INSERT INTO task_number_seq VALUES (NULL)` inside the same
-- transaction as the tasks row insert, then reads the assigned id back via
-- LAST_INSERT_ID() (sql.Result.LastInsertId()) — the same "global,
-- monotonic, not necessarily contiguous per project" guarantee
-- nextval('task.task_number_seq') gives on the Postgres side; project-scoped
-- uniqueness still comes from idx_tasks_project_task_number below, not from
-- this table.
CREATE TABLE task_number_seq (
    id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY
) ENGINE=InnoDB;

-- Postgres's index is a PARTIAL unique index (WHERE task_number IS NOT
-- NULL). Unlike the plain lookup indexes elsewhere in this migration set
-- (idx_tasks_assignee etc.), this one is load-bearing for a UNIQUE
-- constraint, not just a lookup — but the partial-vs-full distinction does
-- NOT change correctness here: a UNIQUE index in both Postgres and MySQL
-- treats every NULL as distinct from every other NULL, so a plain
-- (project_id, task_number) unique index already permits unlimited rows
-- with task_number IS NULL, identical to what the WHERE clause achieves on
-- Postgres by excluding those rows from the index outright. Only the two
-- non-NULL columns actually enforce uniqueness in either dialect.
CREATE UNIQUE INDEX idx_tasks_project_task_number ON tasks (project_id, task_number);
