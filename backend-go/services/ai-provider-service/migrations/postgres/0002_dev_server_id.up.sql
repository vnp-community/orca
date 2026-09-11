-- Closes the dev_server_id implementation-vs-schema drift SOL-005 flags
-- (ai-provider-service.md §5 already documented this column) and adds a
-- soft-delete marker so DeleteAccount (TASK-026) preserves the row for
-- usage_daily's FK and the audit trail instead of a hard DELETE.
--
-- label/model_hint/base_url are added here too: UpdateAccount (TASK-026)
-- is the only usecase that mutates them, and ai-provider-service.md §5
-- already documents all three as part of this table — 0001_init.up.sql
-- just hadn't caught up yet.
ALTER TABLE ai_provider.accounts
  ADD COLUMN dev_server_id TEXT NOT NULL DEFAULT '',
  ADD COLUMN deleted_at TIMESTAMPTZ,
  ADD COLUMN label TEXT NOT NULL DEFAULT '',
  ADD COLUMN model_hint TEXT,
  ADD COLUMN base_url TEXT;
