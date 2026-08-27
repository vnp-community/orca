# TASK-AIP-01-02: Add `0004_outbox` migration (`ai_provider.outbox`)

**From Solution:** SOL-AIP-01
**Priority:** P1
**Service:** `ai-provider-service`
**File:** `backend-go/services/ai-provider-service/migrations/0004_outbox.up.sql` (new)
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

`ai-provider-service.md` §5 already names `ai_provider.outbox` as the
mechanism for lifecycle events (create/rotate/revoke) consumed by other
services. This solution's shape matches `common/outbox.Store`'s actual Go
interface exactly (`id, tenant_id, subject, occurred_at, version, payload,
created_at, published_at`) — the same shape `usage.outbox_events` and
`issuetracking.outbox_events` already use — rather than §5's
`event_type`/`payload`/`created_at`/`published_at`-only sketch. This is a
minor, mechanical divergence flagged in SOL-AIP-01's rationale, not a
design disagreement.

## Changes to make

Create `backend-go/services/ai-provider-service/migrations/0004_outbox.up.sql`:

```sql
CREATE TABLE ai_provider.outbox (
    id           UUID PRIMARY KEY,
    tenant_id    UUID NOT NULL,
    subject      TEXT NOT NULL,
    occurred_at  TIMESTAMPTZ NOT NULL,
    version      INT NOT NULL,
    payload      JSONB NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ
);
CREATE INDEX idx_ai_provider_outbox_unpublished
    ON ai_provider.outbox (created_at) WHERE published_at IS NULL;
```

Create `backend-go/services/ai-provider-service/migrations/0004_outbox.down.sql`:

```sql
DROP TABLE IF EXISTS ai_provider.outbox;
```

## Verify

```bash
cd /opt/repos/orca/backend-go
ls services/ai-provider-service/migrations/0004_outbox.*.sql
go build ./services/ai-provider-service/...
```

Expected: files parse as valid SQL, `up`/`down` round-trip cleanly against
a scratch Postgres, matches the exact column list
`backend-go/services/usage-service/migrations/0002_outbox.up.sql` uses for
`usage.outbox_events` (diff the two files to confirm the shape is
identical modulo schema/table name).
