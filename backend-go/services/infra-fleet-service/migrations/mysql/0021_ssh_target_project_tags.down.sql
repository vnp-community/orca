-- MySQL requires DROP INDEX to be table-scoped, no IF EXISTS support for
-- this form — see migrations/mysql/0003_dev_server_ssh_target.down.sql's
-- same note.
ALTER TABLE ssh_targets DROP INDEX idx_infra_ssh_targets_tenant_host_user;
ALTER TABLE ssh_targets
  DROP COLUMN tags,
  DROP COLUMN project;
