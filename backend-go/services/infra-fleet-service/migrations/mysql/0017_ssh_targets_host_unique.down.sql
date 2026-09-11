-- A UNIQUE table constraint has no separate "constraint" catalog entry in
-- MySQL distinct from its backing index (unlike CHECK/FOREIGN KEY, which
-- MySQL 8.0.19+ can DROP CONSTRAINT by symbol) — dropped as an index instead
-- of Postgres's DROP CONSTRAINT.
ALTER TABLE ssh_targets DROP INDEX ssh_targets_tenant_host_unique;
