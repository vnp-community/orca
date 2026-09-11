-- host is TEXT (unbounded) — MySQL/InnoDB key length is capped at 3072
-- bytes, so a TEXT column used in ANY index (including inside a UNIQUE
-- table constraint) needs an explicit prefix length, unlike Postgres's
-- unbounded-TEXT index. 255 chars covers any real hostname/IP with room to
-- spare (annotation-service's migrations/mysql/0001_init.up.sql set the
-- same precedent for its own TEXT-column composite index).
ALTER TABLE ssh_targets ADD CONSTRAINT ssh_targets_tenant_host_unique UNIQUE (tenant_id, host(255));
