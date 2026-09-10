ALTER TABLE ai_provider.accounts
  ADD COLUMN latency_ms          INTEGER,               -- NULL until first health check
  ADD COLUMN health_detail       TEXT CHECK (health_detail IN
                                   ('healthy','degraded','quota_exceeded','invalid_key','unreachable')),
  ADD COLUMN quota_warning_sent_date DATE;               -- idempotency guard for the 80% alert

CREATE INDEX idx_accounts_due_for_health_check
  ON ai_provider.accounts (last_health_check_at)
  WHERE status = 'active' AND deleted_at IS NULL;

ALTER TABLE ai_provider.usage ADD COLUMN tokens_used BIGINT NOT NULL DEFAULT 0;
