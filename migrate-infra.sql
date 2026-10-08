BEGIN;
UPDATE infra.dev_servers 
SET tenant_id = '8cc62317-8421-43b4-b9d8-38e5fe84bacb' 
WHERE tenant_id = '00000000-0000-0000-0000-000000000001';

UPDATE infra.ssh_targets 
SET tenant_id = '8cc62317-8421-43b4-b9d8-38e5fe84bacb' 
WHERE tenant_id = '00000000-0000-0000-0000-000000000001';
COMMIT;
