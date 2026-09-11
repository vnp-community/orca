DROP INDEX idx_accounts_due_for_health_check ON accounts;
ALTER TABLE accounts
  DROP COLUMN latency_ms,
  DROP COLUMN health_detail,
  DROP COLUMN quota_warning_sent_date;
ALTER TABLE `usage` DROP COLUMN tokens_used;
