# BUG-004: `accounts.*` channels not implemented in backend-go

**Service:** none — no backend-go service manages Claude/Codex CLI account state; this is a backend-host-local capability with no owning gRPC service at all
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go` (absent), `backend-go/services/api-gateway/internal/adapter/wscompat/registry.go` (falls through to `notImplementedHandler`)
**Severity:** Medium — narrow surface (4 methods), but it's the only way a remote-environment user can switch/remove which Claude or Codex CLI account backs their session; without it the account switcher is silently broken whenever `target.kind === 'environment'` (any backend-go-connected dev server/web-mode target).
**Symptom:** `accounts.selectClaude`, `accounts.selectCodex`, `accounts.removeClaude`, `accounts.removeCodex` all time out with `channel "accounts.X" is not yet implemented in backend-go — see backend-go/docs/execution-plan.md's frontend-compatibility-layer coverage table`.
**Status:** ❌ Open

---

## Description

`accounts.*` is the RPC surface behind the Claude/Codex CLI account
switcher — per `specs/frontend/api/rpc-catalog.md:34`, "Claude/Codex account
selection for AI provider auth." It is a distinct concept from
`aiProvider.*` (BUG-005): `accounts.*` manages the Claude/Codex CLI's own
local login state (equivalent to what `claude login`/`codex login` write to
disk), while `aiProvider.*` manages Orca's own provider-credential vault.

None of the 4 methods the frontend calls are registered in
`wscompat.Registry`. Confirmed via:

```
$ grep -n '"accounts\.' backend-go/services/api-gateway/internal/adapter/wscompat/channels.go
(no matches)
```

A repo-wide search for anything account-bridge related in backend-go also
returns nothing:

```
$ grep -rli "claude.*account\|codex.*account\|account.*bridge\|removeClaude\|selectClaude\|selectCodex\|removeCodex" backend-go --include="*.go" --include="*.proto" --include="*.md"
(no matches)
```

`registry.go`'s `NewDefaultServiceRegistry()`
(`backend-go/services/api-gateway/internal/domain/registry.go:82-101`) has no
routing rule for this capability either — consistent with no service owning
it. This is a **service-doesn't-have-this-capability gap**, not a missing
wscompat wrapper over an existing gRPC method: no owning service exists yet
and none of the 17 backend-go services has proto or usecase code that reads
or writes Claude/Codex CLI config files. Whoever picks this up needs to
either add a new endpoint to an existing backend-host-local-capable service
or stand up a new one.

---

## Missing channels

| Method | Frontend call site | Notes |
|---|---|---|
| `accounts.removeClaude` | `frontend/src/renderer/src/runtime/runtime-provider-accounts-client.ts:246-259` (`removeClaudeProviderAccount`, RPC call at line 254) | Params: `{ accountId }`. No owning service. |
| `accounts.removeCodex` | `frontend/src/renderer/src/runtime/runtime-provider-accounts-client.ts:262-275` (`removeCodexProviderAccount`, RPC call at line 270) | Params: `{ accountId }`. No owning service. |
| `accounts.selectClaude` | `frontend/src/renderer/src/runtime/runtime-provider-accounts-client.ts:214-227` (`selectClaudeProviderAccount`, RPC call at line 222) | Params: `{ accountId: selection.accountId }`. No owning service. |
| `accounts.selectCodex` | `frontend/src/renderer/src/runtime/runtime-provider-accounts-client.ts:230-243` (`selectCodexProviderAccount`, RPC call at line 238) | Params: `{ accountId: selection.accountId }`. No owning service. |

Each of these methods is called only when `getActiveRuntimeTarget(settings).kind === 'environment'`
(`runtime-provider-accounts-client.ts:219,235,251,267`) — i.e. only when the
active workspace is backend-go-connected, not local Electron. When
`target.kind !== 'environment'`, the frontend falls back to
`window.api.claudeAccounts`/`window.api.codexAccounts` (Electron IPC, out of
scope for this RPC audit).

There is also a companion `accounts.subscribe` streaming method
(`runtime-provider-accounts-client.ts:143`, used by `watchProviderAccounts`)
that is out of scope for this report's 4-method list per the task's
assignment, but shares the same "no owning service" gap and should be
tracked alongside these 4 if/when this namespace is implemented.

---

## Dispatch model

Per `specs/frontend/api/backend-agent-execution-boundary.md:154`:

> `accounts.*` (mobile Claude/Codex bridge) | 🏠 backend-local | Reads/writes
> the Claude/Codex CLIs' own local config/credential files **on the backend
> host** — distinct from the AI-provider vault on the dev server. No
> Postgres, no relay.

In the old TS backend this was pure backend-host filesystem I/O against the
Claude/Codex CLI's own config/credential files (e.g. the CLI's own
`~/.claude`/`~/.codex`-style login state) — no Postgres table, no relay to
the Dev Server Agent, no cross-service call. Whoever implements this in
backend-go should preserve that shape: a backend-host-local
read/write/select-account operation against the CLI's own on-disk config,
not a database-backed provider-account model (that's what `aiProvider.*`/
`ai-provider-service` already does, and the two must not be conflated —
`accounts.*` never touches `orca_ai_provider_accounts` or
credential-broker-service).

---

## References

- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go` — confirmed no `accounts.*` registrations
- `backend-go/services/api-gateway/internal/adapter/wscompat/registry.go` — `notImplementedHandler`
- `backend-go/services/api-gateway/internal/domain/registry.go:82-101` — `NewDefaultServiceRegistry()`, no accounts-bridge routing rule
- `frontend/src/renderer/src/runtime/runtime-provider-accounts-client.ts:214-275` — `selectClaudeProviderAccount`/`selectCodexProviderAccount`/`removeClaudeProviderAccount`/`removeCodexProviderAccount` call sites
- `specs/frontend/api/backend-agent-execution-boundary.md:154` — `accounts.*` 🏠 backend-local dispatch classification
- `specs/frontend/api/rpc-catalog.md:34,71-78` — `accounts.*` namespace summary and catalog entries
- `specs/backend-go/bugs/missing-v1/BUG-009-files-channels-not-implemented.md` — sibling bug report this follows the format of
