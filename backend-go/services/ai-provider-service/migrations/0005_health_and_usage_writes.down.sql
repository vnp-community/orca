DROP INDEX IF EXISTS ai_provider.idx_accounts_due_for_health_check;
ALTER TABLE ai_provider.accounts
  DROP COLUMN IF EXISTS latency_ms,
  DROP COLUMN IF EXISTS health_detail,
  DROP COLUMN IF EXISTS quota_warning_sent_date;
ALTER TABLE ai_provider.usage DROP COLUMN IF EXISTS tokens_used;
