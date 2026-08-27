# TASK-AIP-03-01: Add `0005_health_and_usage_writes` migration

**From Solution:** SOL-AIP-03
**Priority:** P1
**Service:** `ai-provider-service`
**File:** `backend-go/services/ai-provider-service/migrations/0005_health_and_usage_writes.up.sql` (new)
**Depends on:** TASK-AIP-01-01 (numbered after SOL-AIP-01's `0003`/`0004`; `last_health_check_at`/`quota_limit_day` must already exist)
**Status:** `[ ]` TODO

---

## Context

§8 requires a 15-minute health-check job that updates `status`/
`last_health_check_at`, plus a token-usage write path with quota
enforcement. Neither has the columns it needs yet: `latency_ms`,
`health_detail` (a narrower classification alongside the existing
`status` enum — reusing `status='error'` for any health-check failure,
per SOL-AIP-03's rationale, rather than widening the 5-value lifecycle
enum), `quota_warning_sent_date` (idempotency guard for the 80% alert),
and `ai_provider.usage.tokens_used` (quota enforcement is token-based per
BL-AIP-03; the existing `usage` table only has `cost_usd`/
`request_count`).

## Changes to make

Create `backend-go/services/ai-provider-service/migrations/0005_health_and_usage_writes.up.sql`:

```sql
ALTER TABLE ai_provider.accounts
  ADD COLUMN latency_ms          INTEGER,               -- NULL until first health check
  ADD COLUMN health_detail       TEXT CHECK (health_detail IN
                                   ('healthy','degraded','quota_exceeded','invalid_key','unreachable')),
  ADD COLUMN quota_warning_sent_date DATE;               -- idempotency guard for the 80% alert

CREATE INDEX idx_accounts_due_for_health_check
  ON ai_provider.accounts (last_health_check_at)
  WHERE status = 'active' AND deleted_at IS NULL;

ALTER TABLE ai_provider.usage ADD COLUMN tokens_used BIGINT NOT NULL DEFAULT 0;
```

Create the matching
`backend-go/services/ai-provider-service/migrations/0005_health_and_usage_writes.down.sql`:

```sql
DROP INDEX IF EXISTS ai_provider.idx_accounts_due_for_health_check;
ALTER TABLE ai_provider.accounts
  DROP COLUMN IF EXISTS latency_ms,
  DROP COLUMN IF EXISTS health_detail,
  DROP COLUMN IF EXISTS quota_warning_sent_date;
ALTER TABLE ai_provider.usage DROP COLUMN IF EXISTS tokens_used;
```

`quota_warning_sent_date` is one date value, not a boolean — the usecase
(`TASK-AIP-03-05`) only sends the warning when
`quota_warning_sent_date IS DISTINCT FROM today`, then sets it to today —
naturally resets the next day with no separate cleanup job.

## Verify

```bash
cd /opt/repos/orca/backend-go
ls services/ai-provider-service/migrations/0005_health_and_usage_writes.*.sql
go build ./services/ai-provider-service/...
```

Expected: both files parse as valid SQL and round-trip `up`/`down`
cleanly against a scratch Postgres instance with `0001`-`0004` already
applied.
