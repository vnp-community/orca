# TASK-020: `accountsRelayArgs.AccountID` becomes `*string` to preserve a `null` accountId through the relay

**From Solution:** SOL-010
**Priority:** High
**Service:** `api-gateway`
**File:** `internal/adapter/wscompat/channels_accounts.go`
**Depends on:** none
**Status:** `[x]` DONE — `AccountID` changed from `string` to `*string`; doc comment added explaining why (references BUG-009/SOL-009's live confirmation). No other line in `registerAccountsRelay` changed. `go build`/`go vet` clean for `services/api-gateway`. `impact({target: "registerAccountsRelay", direction: "upstream"})` → `risk: LOW`, 3 impacted symbols, confined to this file's own registration chain — reviewed before editing per this repo's mandatory impact-analysis rule.

---

## Context

See BUG-009's "Root Cause — CONFIRMED" section (superseding its original race-condition filing) and SOL-010's "Why this exists" — a plain `string` field can't distinguish JSON `null` (client's valid "System default" selection) from `""` or an omitted field once `encoding/json` decodes it, silently corrupting every "select System default on a Remote Dev Server" request into an always-rejected one.

## Changes made

`internal/adapter/wscompat/channels_accounts.go`:

Before:
```go
type accountsRelayArgs struct {
	AccountID    string `json:"accountId"`
	ConnectionID string `json:"connectionId"`
}
```

After:
```go
type accountsRelayArgs struct {
	AccountID    *string `json:"accountId"`
	ConnectionID string  `json:"connectionId"`
}
```

(Plus a doc comment on the type explaining the fix — see the file itself.) `registerAccountsRelay`'s body is otherwise byte-for-byte unchanged: `json.Marshal(map[string]any{"accountId": in.AccountID})` already produces the correct `null`/string output once `in.AccountID` is a `*string` — no branching needed.

## Verify

```bash
cd backend-go
go build ./services/api-gateway/...
go vet ./services/api-gateway/...
go test ./services/api-gateway/... -count=1
```

Expected: clean build, all existing tests pass unchanged. TASK-021 adds the regression test specific to this fix.

## Not yet done — deploy

This fix lives in `api-gateway`, which has **not** been rebuilt/redeployed to `b15.openledger.vn` as of this task's completion (BUG-009's earlier SOL-009 fix was `infra-fleet-service` — a different binary; this one needs its own separate deploy). See `deploy/dev/scripts/sync-to-server.sh` for the deploy workflow.
