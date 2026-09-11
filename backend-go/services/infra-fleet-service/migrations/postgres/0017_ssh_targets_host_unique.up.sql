ALTER TABLE infra.ssh_targets ADD CONSTRAINT ssh_targets_tenant_host_unique UNIQUE (tenant_id, host);
