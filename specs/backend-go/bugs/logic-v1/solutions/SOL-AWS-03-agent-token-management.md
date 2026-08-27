# SOL-AWS-03: Persistent, named, per-DevServer agent tokens in Postgres

**Resolves:** [BUG-AWS-03](../BUG-AWS-03-token-management-not-persistent.md)
**Service:** `infra-fleet-service` (primary) + `api-gateway` (wscompat wiring)
**Affected files (proposed):**
- `backend-go/services/infra-fleet-service/migrations/0007_agent_tokens.up.sql` (+ `.down.sql`)
- `backend-go/services/infra-fleet-service/internal/domain/agent_token.go` (new)
- `backend-go/services/infra-fleet-service/internal/usecase/create_agent_token.go`, `list_agent_tokens.go`, `revoke_agent_token.go` (new), `ports.go` (extended)
- `backend-go/services/infra-fleet-service/internal/adapter/postgres/agent_token_repository.go` (new)
- `backend-go/services/infra-fleet-service/internal/adapter/agentwsserver/server.go`, `slots.go`, `token_endpoint.go` (extended, not replaced)
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto` (new RPCs + messages)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_devserver_agent_tokens.go` (new)
- corresponding `*_test.go` files
**Status:** 📋 Proposed — not yet implemented

---

## Design rationale (grounded in TDD)

BUG-AWS-03 documents two token mechanisms today, both in
`backend-go/services/infra-fleet-service/internal/adapter/agentwsserver/`:
`Registry` (`slots.go:24-42`) — an in-memory, single-use, SHA-256-hashed
"pending slot" keyed by token, expiring after `DefaultConnectTimeout` (60s)
— and `TokenIssuer` (`token_endpoint.go:33-49`), an admin-secret-gated
`POST/GET /api/agent-token` that mints those slots and echoes plaintext
metadata (`meta map[string]pendingTokenMeta`) that also dies with the
process. Neither persists past a restart, neither is scoped to more than
one connection attempt, and neither models "10 named tokens per
DevServer" (BL-AWS-03).

`infra-fleet-service.md` §1 states this service "is the system of record
for everything needed to reach a dev server" and §5 is explicit that its
Postgres schema is the durable-state layer for exactly this class of
fact — the doc's own sketch (§5) already lists `dev_servers`,
`ssh_targets`, `connections` as real tables; adding `agent_tokens` as a
sibling table, FK'd to `dev_servers`, is the same pattern this service
already uses for `ssh_targets` (migration `0003_dev_server_ssh_target.up.sql`
links `dev_servers.ssh_target_id`). This is the natural home for BL-AWS-03's
per-DevServer, listable, revocable token set — not a repeat of TS's
"store it in the DevServer's own JSON config file" approach, which doesn't
translate to a relational, multi-replica Go service with no per-service
config file at all (`infra-fleet-service.md` §8: "any pod resolves which
pod owns [a connection]... not shared in-memory state" — the same
argument applies to token state, which must be visible to every replica,
not held in one pod's memory the way `Registry`/`TokenIssuer` do today).

`06-secrets-vault-architecture.md`'s own table draws the line this
solution follows: "User passwords... stores only a bcrypt hash in its own
Postgres (hashes are not secrets in the Vault sense... they don't need to
be retrieved)" vs. every other row in that table, which needs Vault
because the *plaintext* must be retrievable later. A direct-websocket
agent token is the former case — Orca only ever needs to compare an
agent-presented value against a stored hash, never reconstruct the
plaintext itself — so it belongs in `infra-fleet-service`'s own Postgres,
the same class of fact as a password hash, not routed through
`credential-broker-service`. (Contrast with relay-websocket's token, which
Orca must itself *present* outbound — that is a genuinely different case,
addressed in [SOL-AWS-01](./SOL-AWS-01-relay-websocket-per-devserver-token.md),
because sourcing this bug's schema also has to hold that case's row shape.)

## Design — schema

```sql
-- 0007_agent_tokens.up.sql
-- Persistent, named, per-DevServer agent tokens (BL-AWS-03). Coexists with
-- (does not replace) the ephemeral bootstrap Registry/TokenIssuer in
-- adapter/agentwsserver — see this file's usecase-layer doc comments for
-- how the two are reconciled at handshake time.
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

## Design — domain

```go
// internal/domain/agent_token.go
type AgentToken struct {
    ID              string
    TenantID        string
    DevServerID     string
    Name            string
    TokenHash       string // set for direct-websocket rows only
    CredentialRefID string // set for relay-websocket rows only
    CreatedAt       time.Time
    LastUsedAt      *time.Time
    RevokedAt       *time.Time
}

func (t AgentToken) Active() bool { return t.RevokedAt == nil }

var (
    ErrEmptyAgentTokenName    = errors.New("domain: agent token name is required")
    ErrAgentTokenLimitReached = errors.New("domain: a DevServer may have at most 10 active agent tokens")
)

const MaxActiveAgentTokensPerDevServer = 10
```

Plaintext token generation (`crypto/rand`, 32 bytes → 64-char hex, matching
BL-AWS-03's `crypto.randomBytes(32).toString('hex')`) happens in the
**usecase** layer, not the domain constructor — the domain object never
holds plaintext, mirroring `credential-broker-service.md` §4's invariant
that `CredentialMetadata` "has no field capable of holding a secret value."

## Design — usecase

```go
// internal/usecase/create_agent_token.go
type AgentTokenRepository interface {
    CountActive(ctx context.Context, tenantID, devServerID string) (int, error)
    Insert(ctx context.Context, t domain.AgentToken) error
    ListActive(ctx context.Context, tenantID, devServerID string) ([]domain.AgentToken, error)
    FindActiveByHash(ctx context.Context, hash string) (domain.AgentToken, bool, error)
    TouchLastUsed(ctx context.Context, id string) error
    Revoke(ctx context.Context, tenantID, id string) (domain.AgentToken, error)
}

func (uc *CreateAgentToken) Execute(ctx context.Context, tenantID, devServerID, name string) (plaintext string, _ domain.AgentToken, _ error) {
    n, err := uc.repo.CountActive(ctx, tenantID, devServerID)
    if err != nil {
        return "", domain.AgentToken{}, err
    }
    if n >= domain.MaxActiveAgentTokensPerDevServer {
        return "", domain.AgentToken{}, domain.ErrAgentTokenLimitReached
    }
    dev, err := uc.devServers.Get(ctx, tenantID, devServerID)
    if err != nil {
        return "", domain.AgentToken{}, err
    }
    raw := generateHexToken(32) // crypto/rand
    tok := domain.AgentToken{ID: newUUID(), TenantID: tenantID, DevServerID: devServerID, Name: name, CreatedAt: time.Now()}
    switch dev.Mode {
    case domain.ConnectionModeDirectWebSocket:
        tok.TokenHash = sha256Hex(raw)
    case domain.ConnectionModeRelayWebSocket:
        // See SOL-AWS-01: write raw to credential-broker-service, keep only
        // the returned CredentialRef.Id here.
        ref, err := uc.credentialBroker.WriteCredential(ctx, tenantID, devServerID, raw)
        if err != nil {
            return "", domain.AgentToken{}, err
        }
        tok.CredentialRefID = ref.Id
    default:
        return "", domain.AgentToken{}, domain.ErrInvalidConnectionMode // relay-ssh has no agent-token concept (SSH is the trust boundary, §9 of infra-fleet-service.md)
    }
    if err := uc.repo.Insert(ctx, tok); err != nil {
        return "", domain.AgentToken{}, err
    }
    return raw, tok, nil // raw returned ONLY here — never stored, never logged
}
```

`RevokeAgentToken` additionally closes any live session authenticated with
the revoked token, via a new narrow port:

```go
// ports.go addition
type LiveSessionCloser interface {
    // CloseSessionsForDevServerToken closes any direct-websocket session on
    // devServerID currently authenticated as tokenID, with WS close code
    // 1008 and a "token revoked" reason — see SOL-AWS-02 for why 1008, not
    // the never-implemented 4001.
    CloseSessionsForDevServerToken(ctx context.Context, devServerID, tokenID string) (closed int, err error)
}
```

Implemented by `devserveragent.Client` (infra-fleet-service already tracks
one live session per `devServerID`, per `client.go`'s `sessions` map — see
`AttachInboundSession`), so no new session-tracking state is needed.

## Design — reconciling with the existing ephemeral `Registry`/`TokenIssuer`

Both mechanisms coexist, not replace each other, to avoid breaking the
existing "generate a one-shot connect command" bootstrap flow
(`token_endpoint.go`'s `agentCommand` response field):

- `agentwsserver.Server.handleConnection` (`server.go:120-172`) tries
  `Registry.Consume` first (unchanged, for the bootstrap flow), and on a
  miss falls back to a new `TokenValidator` port backed by
  `AgentTokenRepository.FindActiveByHash` — **not single-use**: a
  persistent token stays valid across every reconnect until explicitly
  revoked, matching BL-AWS-03's "USE" step, which describes a repeatable
  compare, not a consume. On a hit, `TouchLastUsed` is called
  best-effort (does not block the handshake on its result).
- `TokenIssuer.handlePost` (`token_endpoint.go:153-208`) is unchanged —
  it remains the admin-secret-gated bootstrap path. The new, user-facing
  "DevServer Settings → Agent Tokens tab" is a **different** surface,
  authenticated as a normal per-tenant admin action (session/JWT +
  OPA, per `07-security-architecture.md`'s AuthZ section), not the
  `ORCA_AGENT_API_SECRET` gate — flagged explicitly since conflating the
  two auth models would be a real regression.

## Design — proto additions (`infrafleet.proto`)

```protobuf
service InfraFleetService {
  // ... existing RPCs ...
  rpc CreateAgentToken(CreateAgentTokenRequest) returns (CreateAgentTokenResponse);
  rpc ListAgentTokens(ListAgentTokensRequest) returns (ListAgentTokensResponse);
  rpc RevokeAgentToken(RevokeAgentTokenRequest) returns (google.protobuf.Empty);
}

message CreateAgentTokenRequest  { string dev_server_id = 1; string name = 2; }
message CreateAgentTokenResponse {
  string id = 1;
  string token = 2; // plaintext — shown once, never returned again
  string name = 3;
  int64 created_at_unix_ms = 4;
}

message AgentTokenSummary {
  string id = 1;
  string name = 2;
  int64 created_at_unix_ms = 3;
  optional int64 last_used_at_unix_ms = 4;
}
message ListAgentTokensRequest  { string dev_server_id = 1; }
message ListAgentTokensResponse { repeated AgentTokenSummary tokens = 1; }

message RevokeAgentTokenRequest { string dev_server_id = 1; string id = 2; }
```

## Design — wscompat wiring

```go
// channels_devserver_agent_tokens.go
func registerDevServerAgentTokenChannels(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
    r.Register("devServer.agentTokens.create", ...) // -> CreateAgentToken
    r.Register("devServer.agentTokens.list", ...)   // -> ListAgentTokens
    r.Register("devServer.agentTokens.revoke", ...) // -> RevokeAgentToken
}
```

Same shape as `registerAccountsChannels` (`channels_accounts.go`) — decode
args, attach identity, call the RPC, map the response — no new pattern
introduced.

## Test plan

- `domain/agent_token_test.go` — `MaxActiveAgentTokensPerDevServer` invariant, `exactly_one_secret_ref`-equivalent construction guard.
- `usecase/create_agent_token_test.go` — 11th active token rejected with `ErrAgentTokenLimitReached`; direct-websocket rows get `TokenHash` only; relay-websocket rows call `credentialBroker.WriteCredential` and store `CredentialRefID` only; plaintext never appears in any field read back from a fake repo.
- `usecase/revoke_agent_token_test.go` — revoke sets `RevokedAt`, calls `LiveSessionCloser.CloseSessionsForDevServerToken` exactly once.
- `adapter/postgres/agent_token_repository_test.go` — `FindActiveByHash` excludes revoked rows; `CountActive` excludes revoked rows; RLS smoke test (tenant B cannot see tenant A's rows).
- `adapter/agentwsserver/server_test.go` — handshake succeeds against a persistent (non-`Registry`) token, and succeeds again on a **second** handshake with the same token (proves non-single-use); a revoked token's handshake is rejected.
- `wscompat/channels_devserver_agent_tokens_test.go` — one test per channel using a fake `InfraFleetServiceClient`, plus a test asserting the plaintext token from `create` is never re-derivable from `list`'s response shape.

## References

- `backend-go/services/infra-fleet-service/internal/adapter/agentwsserver/slots.go:24-42` — existing ephemeral `Registry`
- `backend-go/services/infra-fleet-service/internal/adapter/agentwsserver/token_endpoint.go:33-49,153-208` — existing `TokenIssuer`
- `backend-go/services/infra-fleet-service/migrations/0001_init.up.sql`, `0003_dev_server_ssh_target.up.sql` — migration style this solution follows
- `specs/backend-go/tdd/services/infra-fleet-service.md:174-283` (§5 data model), `:446-484` (§8, multi-replica/no-shared-memory argument)
- `specs/backend-go/tdd/architecture/06-secrets-vault-architecture.md:17-30` (what goes in Vault vs. Postgres — the password-hash precedent this solution's direct-websocket branch follows)
- `docs/logic/agent-ws/BL-AWS-03-token-management.md:14-44` — token lifecycle/storage shape
- [SOL-AWS-01](./SOL-AWS-01-relay-websocket-per-devserver-token.md) — the relay-websocket/`credential_ref_id` half of this schema
- [SOL-AWS-02](./SOL-AWS-02-direct-websocket-protocol-divergence.md) — close code 1008 used by `RevokeAgentToken`'s live-session close
