-- No-op: device_id CHAR(36) is already part of push_subscriptions in
-- 0001_init.up.sql for this dialect (see that file's comment — the MySQL
-- rollout starts from a single combined baseline, so this migration exists
-- only to keep the up/down migration NUMBERING aligned 1:1 with the
-- Postgres timeline, not to apply a schema change of its own).
SELECT 1;
