-- The user who started the execution (see the Postgres migration of the same number for why).
ALTER TABLE executions ADD COLUMN started_by VARCHAR(64) NULL;
