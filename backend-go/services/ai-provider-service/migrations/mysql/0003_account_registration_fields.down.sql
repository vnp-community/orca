DROP INDEX uq_accounts_one_default_per_dev_server_provider ON accounts;
ALTER TABLE accounts
  DROP COLUMN default_slot_key,
  DROP COLUMN quota_limit_day,
  DROP COLUMN last_health_check_at,
  DROP COLUMN created_by,
  DROP COLUMN models,
  DROP COLUMN is_default;
