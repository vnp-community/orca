# TASK-INT-03-01: Add `PreflightCheckResult` type + `MergePreflightStatuses`

**From Solution:** SOL-INT-03
**Priority:** P1
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/usecase/preflight.go` (new)
**Depends on:** none
**Status:** `[x]` DONE — usecase/preflight.go created with PreflightCheckResult/MergePreflightStatuses; preflight_test.go covers relay-overrides-local, relay-only-append, relayErr-produces-connectivity-warning-only-even-with-nonempty-relay, stable ordering, and empty inputs — all pass.

---

## Context

BL-INT-03's `mergePreflightStatuses()` — the actual algorithmic core this
bug centers on — has no Go port anywhere. This task adds it in isolation
(pure function, no gRPC dependency) so it's independently testable before
`TASK-INT-03-03` wires it into `wscompat`.

## Changes to make

Create `backend-go/services/api-gateway/internal/usecase/preflight.go`:

```go
package usecase

// PreflightStatus is one check's outcome — mirrors BL-INT-03's status enum.
type PreflightStatus string

const (
	PreflightOK      PreflightStatus = "ok"
	PreflightWarning PreflightStatus = "warning"
	PreflightError   PreflightStatus = "error"
	PreflightSkip    PreflightStatus = "skip"
)

// PreflightSource distinguishes a locally-computed check from one relayed
// through a Dev Server — UI-relevant (e.g. "this needs the Dev Server to be
// reachable") and a regression guard: every result MUST be tagged.
type PreflightSource string

const (
	PreflightSourceLocal PreflightSource = "local"
	PreflightSourceRelay PreflightSource = "relay"
)

// PreflightCheckResult is one named check's result — the Go port of
// BL-INT-03's PreflightCheckResult schema.
type PreflightCheckResult struct {
	ID      string
	Status  PreflightStatus
	Message string
	Details map[string]any
	Source  PreflightSource
}

// MergePreflightStatuses seeds by id from local, then overrides by id from
// relay (relay wins conflicts, matching BL-INT-03's documented
// precedence), appends any relay-only ids, and appends a
// "relay-connectivity" warning when relayErr is non-nil INSTEAD OF any
// relay results at all — even if a non-empty relay slice was also passed,
// a non-nil relayErr wins ("local checks only" is the only honest answer
// when connectivity itself failed). Output order is stable: local order,
// then relay-only appends, then the connectivity warning (if any) — for
// deterministic UI rendering.
func MergePreflightStatuses(local, relay []PreflightCheckResult, relayErr error) []PreflightCheckResult {
	merged := make(map[string]PreflightCheckResult, len(local)+len(relay))
	order := make([]string, 0, len(local)+len(relay)+1)
	for _, c := range local {
		merged[c.ID] = c
		order = append(order, c.ID)
	}

	if relayErr != nil {
		order = append(order, "relay-connectivity")
		merged["relay-connectivity"] = PreflightCheckResult{
			ID: "relay-connectivity", Status: PreflightWarning,
			Message: "Cannot reach Dev Server — showing local checks only",
			Source:  PreflightSourceLocal,
		}
		out := make([]PreflightCheckResult, 0, len(order))
		for _, id := range order {
			out = append(out, merged[id])
		}
		return out
	}

	for _, c := range relay {
		if _, existed := merged[c.ID]; !existed {
			order = append(order, c.ID)
		}
		merged[c.ID] = c // relay overrides local by id
	}
	out := make([]PreflightCheckResult, 0, len(order))
	for _, id := range order {
		out = append(out, merged[id])
	}
	return out
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/api-gateway/internal/usecase/...
```

Expected: clean build. Add `preflight_test.go`: relay overrides local by
id; relay-only ids are appended; a non-nil `relayErr` produces exactly the
`relay-connectivity` warning and **no** relay results, even when a
non-empty `relay` slice was passed (the "local checks only" contract);
output order is stable (local order, then relay-only appends, then the
warning).
