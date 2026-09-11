-- MySQL requires DROP INDEX to be table-scoped (no schema-qualified bare
-- "DROP INDEX name" like Postgres) — combined into one ALTER TABLE with
-- the column drop, which also implicitly removes the index in one step,
-- but dropping it explicitly first keeps this parallel to the Postgres
-- variant's two-statement shape.
ALTER TABLE dev_servers DROP INDEX idx_infra_dev_servers_ssh_target;
ALTER TABLE dev_servers DROP COLUMN ssh_target_id;
