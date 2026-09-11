-- DROP INDEX (not DROP CONSTRAINT) for maximum MySQL 8.0.x compatibility —
-- a UNIQUE constraint is implemented as an index, and DROP CONSTRAINT for
-- non-CHECK constraints was only added in MySQL 8.0.19.
DROP INDEX tasks_share_token_unique ON tasks;
ALTER TABLE tasks DROP COLUMN share_token;

ALTER TABLE tasks
  DROP COLUMN total_subtasks,
  DROP COLUMN done_subtasks,
  DROP COLUMN workflow_exec_id,
  DROP COLUMN reporter_id,
  DROP COLUMN labels;
