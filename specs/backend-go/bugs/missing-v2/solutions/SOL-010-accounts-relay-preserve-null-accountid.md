# SOL-010: `accounts.selectClaude`/`accounts.selectCodex` preserve a `null` accountId instead of collapsing it to `""`

**Resolves:** BUG-009 (the real root cause, confirmed 2026-09-14 via SOL-009's cause-logging — supersedes that file's original race-condition theory)
**Service:** `api-gateway`
**Affected files:** `internal/adapter/wscompat/channels_accounts.go`, `internal/adapter/wscompat/channels_accounts_test.go`
**Priority:** High
**Status:** ✅ IMPLEMENTED (2026-09-14) — TASK-020 + TASK-021. **Not yet deployed** — see BUG-009's "Deploy status".

---

## Why this exists

`accountsRelayArgs.AccountID` was declared as a plain Go `string`:

```go
type accountsRelayArgs struct {
	AccountID    string `json:"accountId"`
	ConnectionID string `json:"connectionId"`
}
```

`encoding/json` unmarshals JSON `null` into a `string` field as the zero value `""` — there is no way to distinguish "the client explicitly sent `null`" from "the client sent `\"\"`" or "the field was omitted" once decoded into a plain `string`. The dev server agent's own `parseSelectAccountId` (`agent/src/relay/accounts-handler.ts`) treats these as **three different, meaningfully different** cases:

| Wire value | Agent's `parseSelectAccountId` result |
|---|---|
| `null` | valid — "System default" / deselect |
| `""` (empty string) | `INVALID` — `.trim()` is falsy |
| key absent entirely | `INVALID` — `!('accountId' in params)` |

`registerAccountsRelay`'s old `string`-typed field could only ever produce the middle row after decoding, no matter which of the three the client actually sent — so a client sending the first (valid) case had it silently corrupted into the second (always-rejected) case before ever reaching the agent.

## Design

```go
// channels_accounts.go
type accountsRelayArgs struct {
	AccountID    *string `json:"accountId"`
	ConnectionID string  `json:"connectionId"`
}
```

No other line in `registerAccountsRelay` changes. `json.Marshal(map[string]any{"accountId": in.AccountID})` — with `in.AccountID` now a `*string` — already does the right thing for both cases without any extra branching:

- `in.AccountID == nil` (client sent JSON `null`) → marshals to `"accountId": null` → agent's `parseSelectAccountId` takes the `null`-is-valid branch.
- `in.AccountID` pointing at a real string → marshals to `"accountId": "<value>"` → unchanged from before.

This is the smallest possible fix: one field's type, zero behavior change to the marshal/relay logic already there, and — per `*string` being an already-established idiom in this same `wscompat` package (`channels_automation_task.go`, `channels_cli_installer.go` already use `*string` for optional/nullable wire fields) — no new pattern introduced to the codebase.

### Why not fix it differently

- **Fixing it in the frontend** (never send `null`, send some sentinel string instead) would require the agent to also change its `parseSelectAccountId` contract, and would leak a wire-format workaround into product code for what is purely a Go-decoding limitation in one relay hop.
- **Fixing it in the agent** (treat `""` the same as `null`) would blur a distinction the agent's own code deliberately keeps (an explicitly-sent empty string arguably SHOULD be rejected as a caller mistake, separately from "the caller means System default") — and wouldn't fix `removeClaude`/`removeCodex`'s similar exposure if the frontend ever needed a null case there too.
- Fixing the actual data-corruption point (`api-gateway`'s decode step) is the one change that makes the whole chain correct for every current and future caller of this shared struct, not just today's 2 call sites that happen to hit it.

## Testing

`channels_accounts_test.go` (existing file, extended):
- `TestAccountsChannels_NullAccountId_RelaysAsJSONNull` (new) — sends `{"accountId": nil, "connectionId": "ds-1"}` through `accounts.selectClaude` and `accounts.selectCodex`, asserts the JSON forwarded to `RelayByDevServer`'s `ParamsJson` has `"accountId"` present and equal to `nil` (JSON `null`), not `""`. **Proven to actually catch the bug**: reverted the `*string` fix locally, confirmed this test fails with `relayed accountId = "", want JSON null`, then restored the fix — not just "it compiles."
- All 11 pre-existing tests in this file (`TestAccountsChannels_RelaySuccess`, `TestAccountsResolveDevServerConnection_*`, etc.) still pass unchanged — this fix doesn't touch `removeClaude`/`removeCodex`'s existing (always-non-null) behavior or any other channel in this file.

## Verify

```bash
cd backend-go
go build ./services/api-gateway/...
go vet ./services/api-gateway/...
go test ./services/api-gateway/internal/adapter/wscompat/... -run TestAccounts -count=1 -v
```

Expected: clean build, all tests pass (12 total in this file after this task, up from 11).
