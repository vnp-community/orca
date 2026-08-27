# TASK-AWS-03-01: Add `infra.agent_tokens` migration

**From Solution:** SOL-AWS-03
**Priority:** P0 — every other AWS-03/AWS-01 task depends on this table existing
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/migrations/0007_agent_tokens.up.sql` (+ `.down.sql`)
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

BL-AWS-03 needs persistent, named, per-DevServer agent tokens (up to 10
active per DevServer). Today's `Registry`/`TokenIssuer`
(`adapter/agentwsserver/slots.go`, `token_endpoint.go`) are in-memory only
and die with the process. This migration adds the durable table both
SOL-AWS-03 (direct-websocket `token_hash` rows) and SOL-AWS-01
(relay-websocket `credential_ref_id` rows) write to, following the existing
`0003_dev_server_ssh_target.up.sql`/`0006_browser_profiles.up.sql` migration
style (FK to `infra.dev_servers`, RLS policy).

## Changes to make

Create `backend-go/services/infra-fleet-service/migrations/0007_agent_tokens.up.sql`:

```sql
-- Persistent, named, per-DevServer agent tokens (BL-AWS-03). Coexists with
-- (does not replace) the ephemeral bootstrap Registry/TokenIssuer in
-- adapter/agentwsserver — see usecase/create_agent_token.go's doc comment
-- for how the two are reconciled at handshake time.
CREATE TABLE infra.agent_tokens (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id         UUID NOT NULL,
    dev_server_id     UUID NOT NULL REFERENCES infra.dev_servers(id),
    name              TEXT NOT NULL,
    -- Exactly one of token_hash / credential_ref_id is set, depending on
    -- the owning dev_server's connection_mode — see SOL-AWS-01 for why
    -- relay-websocket's row can't be a bare hash (Orca must itself present
    -- the plaintext outbound, so that case's secret lives in
    -- credential-broker-service/Vault, referenced here by id only).
    token_hash        TEXT,          -- SHA-256 hex, direct-websocket only
    credential_ref_id UUID,          -- credential-broker-service CredentialMetadata.id, relay-websocket only
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at      TIMESTAMPTZ,
    revoked_at        TIMESTAMPTZ,

    CONSTRAINT exactly_one_secret_ref CHECK (
        (token_hash IS NOT NULL AND credential_ref_id IS NULL) OR
        (token_hash IS NULL AND credential_ref_id IS NOT NULL)
    )
);

CREATE UNIQUE INDEX idx_agent_tokens_hash ON infra.agent_tokens (token_hash)
    WHERE token_hash IS NOT NULL;
CREATE INDEX idx_agent_tokens_dev_server_active ON infra.agent_tokens (dev_server_id)
    WHERE revoked_at IS NULL;

ALTER TABLE infra.agent_tokens ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON infra.agent_tokens
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
```

Create `backend-go/services/infra-fleet-service/migrations/0007_agent_tokens.down.sql`:

```sql
DROP TABLE IF EXISTS infra.agent_tokens;
```

## Verify

```bash
cd /opt/repos/orca/backend-go
ls services/infra-fleet-service/migrations/ | grep 0007
# run against a local/test Postgres if the service's migration runner is available:
go run ./services/infra-fleet-service/cmd/server -- --migrate-only 2>/dev/null || true
```

Expected: migration files present, numbered after `0006_browser_profiles`;
if a migration-check tool/CI step exists for this service, it reports no
syntax errors and the `up`/`down` pair round-trips cleanly.
