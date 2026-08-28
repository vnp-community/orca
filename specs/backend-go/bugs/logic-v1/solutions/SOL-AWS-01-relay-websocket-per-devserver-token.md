# SOL-AWS-01: Per-DevServer relay-websocket tokens via credential-broker-service, not `ORCA_AGENT_TOKEN`

**Resolves:** [BUG-AWS-01](../BUG-AWS-01-relay-websocket-single-shared-token.md)
**Service:** `infra-fleet-service` (dial-out) + `credential-broker-service` (secret custody)
**Affected files (proposed):**
- `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/config.go`, `client.go`, `session.go` (extended)
- `backend-go/services/infra-fleet-service/internal/usecase/ports.go` (extend with `CredentialBrokerClient`)
- `backend-go/proto/orca/credentialbroker/v1/credentialbroker.proto` (extend `CredentialCategory`)
- Shared schema with [SOL-AWS-03](./SOL-AWS-03-agent-token-management.md)'s `infra.agent_tokens` table (`credential_ref_id` column)
- corresponding `*_test.go` files
**Status:** 📋 Proposed — not yet implemented

---

## Design rationale (grounded in TDD)

BUG-AWS-01 is accurate that `devserveragent.Config.Token`
(`config.go:9-33`) is one process-wide value sourced from `ORCA_AGENT_TOKEN`
(`LoadConfigFromEnv`, `config.go:75-92`), reused for every relay-websocket
`DevServer` the process ever dials, and that `domain.DevServer`
(`dev_server.go:53-59`) has no token field at all.

**Why this is not simply "add a `TokenHash` column and look it up," the
way direct-websocket's token is fixed in SOL-AWS-03**: relay-websocket's
`Authorization: Bearer <agentToken>` header (`session.go:112,120`) is sent
by Orca *to* the agent — Orca is the WS **client** in this mode
(`infra-fleet-service.md` §6: `directws/` "dial out"; BL-AWS-01: "Orca
(client) kết nối tới... Agent (server)"). Orca must retain a value it can
*present*, not merely compare against a locally-computed hash. A SHA-256
hash — suficient for direct-websocket, where Orca is the verifier
(SOL-AWS-03) — is structurally useless here: `SHA-256` is one-way, so a
hash-only row could never be turned back into the bearer value Orca has to
send on the next dial.

This is exactly the class of secret `06-secrets-vault-architecture.md`'s
table assigns to Vault, not to a service's own Postgres: "SSH credentials
for dev servers... removes long-lived SSH private keys from
`infra-fleet-service`'s filesystem/database entirely," and
`infra-fleet-service.md` §9 states the same for `ssh_targets.auth_vault_path`
— "no long-lived SSH private keys on this service's filesystem or database,
ever... a pointer, never key material." A relay-websocket agent token is
functionally the same shape of fact as an SSH credential in this service's
own model: a secret `infra-fleet-service` must hold *live* in order to
authenticate itself outbound to a target it doesn't control. The existing
`ssh_targets.auth_vault_path` pattern (pointer column, secret in Vault) is
the direct precedent this solution follows, mediated through
`credential-broker-service` per that service's own "no other service talks
to Vault directly for tenant secret material" rule
(`credential-broker-service.md` §2, reasons 1–3) — `infra-fleet-service`'s
own Vault-direct exception (§9's SSH secrets engine / KV v2 access) is
scoped to *SSH* credentials specifically and is flagged in that doc as
needing confirmation against `06-secrets-vault-architecture.md`'s
default ("no other service talks to Vault directly"); this solution does
not extend that exception to agent tokens and instead routes through the
broker, the less surprising reading of the two docs together.

**Category gap, flagged as a scope addition**: `credentialbroker.proto`'s
`CredentialCategory` enum (`:69-76`) has five values, none of which fit a
relay-websocket agent bearer token — `SSH` is semantically an SSH
credential (cert/key), not a bearer token for a bespoke WS handshake, and
reusing it would make `ResolveCredential`'s category-to-engine branching
(`credential-broker-service.md` §4's invariant: "`VaultEngine` is derived
from `Category`... prevents a caller from requesting a category/engine
mismatch") incoherent for a value that isn't actually SSH material. This
solution proposes a sixth category:

```protobuf
enum CredentialCategory {
  // ... existing 5 values ...
  CREDENTIAL_CATEGORY_DEV_SERVER_AGENT_TOKEN = 6; // relay-websocket Authorization: Bearer token, see SOL-AWS-01
}
```

Mapped to Vault KV v2 (static secret, versioned, matches
`06-secrets-vault-architecture.md`'s "Vault engines used" table row for
"Webhook signing secrets, service-to-service shared secrets" — the closest
existing category by shape: a static bearer credential this service
presents to an external endpoint it doesn't control, not something Vault
signs fresh per connection the way an SSH certificate is).

## Design — write path (token creation)

Handled by [SOL-AWS-03](./SOL-AWS-03-agent-token-management.md)'s
`CreateAgentToken` usecase for a `DevServer` in `relay-websocket` mode:
generates the 64-char hex value locally, calls
`credentialBroker.WriteCredential(ctx, tenantID, ownerID=devServerID,
category=DEV_SERVER_AGENT_TOKEN, envelope=rawToken)`, and stores only the
returned `CredentialRef.Id` in `infra.agent_tokens.credential_ref_id` —
the plaintext is never written to `infra-fleet-service`'s own database,
matching `credential-broker-service.md`'s "plaintext exists ONLY inside
this activation" guarantee (§9).

`WriteCredentialRequest.encrypted_envelope` (`credentialbroker.proto:87`)
is documented as a **client-side-encrypted** envelope for the
browser-facing write path (AI provider keys) — this call site is
service-to-service, not browser-originated, so there is no transport-layer
envelope to decrypt first; `infra-fleet-service` sends the raw token bytes
directly over the mTLS-secured internal mesh, the same posture
`WriteCredentialRequest`'s own doc comment carves out for
`SERVICE_SECRET`/`VAPID_KEY` ("plaintext over the mTLS-secured internal
mesh, since there is no browser-facing transport leg to begin with") —
`DEV_SERVER_AGENT_TOKEN` is the same case, flagged here as an explicit
extension of that existing carve-out to a new category.

## Design — dial-out path (`adapter/devserveragent`)

```go
// usecase/ports.go addition
type CredentialBrokerClient interface {
    ResolveCredential(ctx context.Context, credentialRefID string) ([]byte, error)
}
```

```go
// adapter/devserveragent/client.go — getOrDialSession, relay-websocket branch
func (c *Client) tokenFor(ctx context.Context, devServer domain.DevServer) (string, error) {
    ref, ok, err := c.agentTokens.ActiveForDevServer(ctx, devServer.TenantID, devServer.ID)
    if err != nil {
        return "", err
    }
    if !ok {
        return "", fmt.Errorf("devserveragent: no active relay-websocket token registered for dev server %s", devServer.ID)
    }
    raw, err := c.credentialBroker.ResolveCredential(ctx, ref.CredentialRefID)
    if err != nil {
        return "", fmt.Errorf("devserveragent: resolving agent token: %w", err)
    }
    return string(raw), nil
}
```

`Config.Token`/`ORCA_AGENT_TOKEN` (`config.go:16-24,86`) is removed as the
per-connection source of truth; `Config` keeps only the transport-tuning
fields (`Port`, `DialTimeout`, etc. — unaffected). A deployment with no
registered per-DevServer token for a given `DevServer` fails that dial
attempt with a clear, actionable error instead of silently falling back to
a shared secret — no silent compatibility shim, since the whole point of
this fix is that a shared secret must stop being usable to reach a
DevServer whose token was revoked.

`ActiveForDevServer` (picks the most-recently-created non-revoked row for
that DevServer) is a new read method on
[SOL-AWS-03](./SOL-AWS-03-agent-token-management.md)'s
`AgentTokenRepository` — relay-websocket DevServers are expected to carry
exactly one *active* token in ordinary operation (the "10 named tokens"
cap from BL-AWS-03 exists for the admin UI's flexibility — e.g. rotating
between a "prod" and "staging" agent token — not because Orca dials with
more than one at a time per connection attempt).

**Resolve on every dial, not once at startup.** Because `ResolveCredential`
is called per dial (not cached across process restarts), a revoked token
is honored on the very next reconnect attempt with no code/config deploy —
same guarantee `credential-broker-service.md` §9 ("Immediate revocation
without a deploy") already promises for every other credential category;
this closes BUG-AWS-01's "no revoke path" finding as a direct consequence
of the write-path fix, not a separate mechanism.

## Test plan

- `adapter/devserveragent/client_test.go` — dialing a relay-websocket
  `DevServer` with no registered token fails with a clear error, no dial
  attempted; dialing with a registered token calls
  `CredentialBrokerClient.ResolveCredential` with the stored
  `credential_ref_id` and sends the resolved plaintext as the `Bearer`
  header (assert against a fake WS server capturing the header, extending
  the existing `session.go:112,120` header-construction test).
- `adapter/devserveragent/client_test.go` — two different `DevServer`s with
  two different tokens produce two different `Authorization` headers
  (regression guard against the shared-token bug this solution fixes).
- `usecase/create_agent_token_test.go` (extends SOL-AWS-03's test) —
  relay-websocket branch calls `WriteCredential` with
  `CREDENTIAL_CATEGORY_DEV_SERVER_AGENT_TOKEN` and never writes
  `TokenHash`.
- Integration/contract test: revoking a relay-websocket token
  (`RevokeCredentialByOwner` or `RevokeCredential`) causes the *next* dial
  attempt to fail closed — no process restart required.

## References

- `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/config.go:9-33,75-92` — `Config.Token`/`LoadConfigFromEnv`, replaced by per-dial resolution
- `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/session.go:112,120` — `Authorization: Bearer` header construction, unchanged shape, new value source
- `specs/backend-go/tdd/services/infra-fleet-service.md:485-524` (§9 security notes — `ssh_targets.auth_vault_path` precedent, §9's own flagged Vault-direct-access caveat)
- `specs/backend-go/tdd/architecture/06-secrets-vault-architecture.md:17-30` (Vault-vs-Postgres table), `:31-65` (`credential-broker-service`'s mediation rule and its DB-credential exception)
- `specs/backend-go/tdd/services/credential-broker-service.md:118-177` (§3 API surface, `WriteCredentialRequest`'s browser-vs-service-mesh envelope distinction), `:188-214` (§4 `VaultEngine`-derived-from-`Category` invariant)
- `backend-go/proto/orca/credentialbroker/v1/credentialbroker.proto:69-76` — `CredentialCategory` enum extended by this solution
- [SOL-AWS-03](./SOL-AWS-03-agent-token-management.md) — shared `infra.agent_tokens` schema, `credential_ref_id` column, `CreateAgentToken` usecase this solution's write path reuses
