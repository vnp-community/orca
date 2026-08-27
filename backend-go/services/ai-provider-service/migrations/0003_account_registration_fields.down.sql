DROP INDEX IF EXISTS ai_provider.uq_accounts_one_default_per_dev_server_provider;
ALTER TABLE ai_provider.accounts
  DROP COLUMN IF EXISTS quota_limit_day,
  DROP COLUMN IF EXISTS last_health_check_at,
  DROP COLUMN IF EXISTS created_by,
  DROP COLUMN IF EXISTS models,
  DROP COLUMN IF EXISTS is_default;
