# TASK-AIP-02-03: Add `ProviderType` filter to `ListAccountsFilter`

**From Solution:** SOL-AIP-02
**Priority:** P0 — correctness bug
**Service:** `ai-provider-service`
**File:** `backend-go/services/ai-provider-service/internal/usecase/ports.go`
**Depends on:** none
**Status:** `[x] DONE — ListAccountsFilter.ProviderType added; go build/vet clean.`

---

## Context

`ListAccountsFilter` (`ports.go:24-29`) already has `DevServerID` — never
set by `resolve_provider.go` today (see `TASK-AIP-02-05`) — but has no
`ProviderType` field at all, so there is no way for any caller to narrow a
`List` call to one provider. This is the port-level half of BUG-AIP-02's
fix; `TASK-AIP-02-04` wires the corresponding SQL clause.

## Changes to make

In `backend-go/services/ai-provider-service/internal/usecase/ports.go`,
replace:

```go
type ListAccountsFilter struct {
	TenantID     string
	Scope        domain.AccountScope // zero value = any scope
	ScopeRefID   string
	DevServerID  string               // optional — empty means no filter
	ProviderType domain.ProviderType  // NEW — zero value = any provider, matches DevServerID's "empty = no filter" convention
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/ai-provider-service/...
go vet ./services/ai-provider-service/...
```

Expected: clean build. No behavior changes yet — `repository.go`'s SQL
(`TASK-AIP-02-04`) doesn't reference the new field until that task lands,
so `List` silently ignores it until then; that's expected and resolved in
the next task.
