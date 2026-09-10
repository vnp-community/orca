-- Placeholder counterpart to 0007_dev_server_health_status.up.sql — see
-- that file's comment. Intentionally a no-op: this branch never applied
-- the real version-7 change, so it has nothing of its own to revert.
-- Running this down migration will NOT restore the pre-migration schema —
-- whoever owns the real 0007 must supply the real down migration.
SELECT 1;
