# TASK-AIP-01-01: Add `0003_account_registration_fields` migration

**From Solution:** SOL-AIP-01
**Priority:** P1
**Service:** `ai-provider-service`
**File:** `backend-go/services/ai-provider-service/migrations/0003_account_registration_fields.up.sql` (new)
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

`domain.ProviderAccount` is missing 5 columns BL-AIP-01 needs:
`quota_limit_day`, `last_health_check_at`, `created_by` (already speced in
`ai-provider-service.md` §4/§5 but never migrated), plus `models` and
`is_default` (genuine extensions beyond §5's sketch — see SOL-AIP-01's
rationale, Tier 2). This task adds the schema only; domain/repo code lands
in later tasks.

## Changes to make

Create `backend-go/services/ai-provider-service/migrations/0003_account_registration_fields.up.sql`:

```sql
ALTER TABLE ai_provider.accounts
  ADD COLUMN quota_limit_day      INTEGER NOT NULL DEFAULT 0,  -- 0 = unlimited, per ai-provider-service.md §5
  ADD COLUMN last_health_check_at TIMESTAMPTZ,
  ADD COLUMN created_by           UUID,
  -- Not in §5's original sketch — added per SOL-AIP-01's rationale
  -- (BUG-AIP-02's Models-list dependency, BL-AIP-02's server-default tier).
  ADD COLUMN models               TEXT[] NOT NULL DEFAULT '{}',
  ADD COLUMN is_default           BOOLEAN NOT NULL DEFAULT false;

-- At most one default per (tenant, dev_server, provider_type) — enforced at
-- the DB level, mirroring credential-broker-service's unique_vault_path
-- posture (defense in depth, not "trust the usecase layer alone").
CREATE UNIQUE INDEX uq_accounts_one_default_per_dev_server_provider
  ON ai_provider.accounts (tenant_id, dev_server_id, provider_type)
  WHERE is_default AND deleted_at IS NULL;

-- quota_limit_day >= 1000 (BL-AIP-01's field rule) is enforced in the domain
-- constructor, not a CHECK constraint — 0 (unlimited) must stay legal, and
-- "no lower than 1000 unless 0" isn't cleanly expressible as one CHECK
-- clause without duplicating the domain's own validation decision.
```

Create the matching `backend-go/services/ai-provider-service/migrations/0003_account_registration_fields.down.sql`:

```sql
DROP INDEX IF EXISTS ai_provider.uq_accounts_one_default_per_dev_server_provider;
ALTER TABLE ai_provider.accounts
  DROP COLUMN IF EXISTS quota_limit_day,
  DROP COLUMN IF EXISTS last_health_check_at,
  DROP COLUMN IF EXISTS created_by,
  DROP COLUMN IF EXISTS models,
  DROP COLUMN IF EXISTS is_default;
```

## Verify

```bash
cd /opt/repos/orca/backend-go
ls services/ai-provider-service/migrations/0003_account_registration_fields.*.sql
# Run against a scratch DB if migrate tooling is available locally, e.g.:
# migrate -path services/ai-provider-service/migrations -database "$TEST_DATABASE_DSN" up
# migrate -path services/ai-provider-service/migrations -database "$TEST_DATABASE_DSN" down 1
go build ./services/ai-provider-service/...
```

Expected: both files parse as valid SQL (no syntax errors), `up` then
`down` round-trips cleanly against a scratch Postgres instance, and
`go build` stays green (this task doesn't touch Go code, so no behavior
change yet).
