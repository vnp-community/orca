# TASK-AT-01-02: `Automation` domain gains `ProjectID`/`Actions`; new `AutomationAction`/`OnFailurePolicy` types

**From Solution:** SOL-AT-01
**Priority:** P0
**Service:** `automation-service`
**File:** `backend-go/services/automation-service/internal/domain/automation_action.go` (new), `backend-go/services/automation-service/internal/domain/automation.go`
**Depends on:** TASK-AT-01-01
**Status:** `[ ]` TODO

---

## Context

The domain model must carry `ProjectID` and an ordered `Actions` chain so
`NewAutomation` can validate BR-AT-01 (non-empty action list). This replaces
the single `StepType`/`StepConfigJSON` pair as the source of truth for new
rows while keeping the old fields for back-compat reads.

## Changes to make

Create `backend-go/services/automation-service/internal/domain/automation_action.go`:

```go
package domain

// OnFailurePolicy controls whether RunNow's action loop continues to the
// next action or stops the run when one action fails.
type OnFailurePolicy string

const (
	OnFailureStop     OnFailurePolicy = "stop"
	OnFailureContinue OnFailurePolicy = "continue"
)

// Valid reports whether p is one of the two known policies.
func (p OnFailurePolicy) Valid() bool {
	switch p {
	case OnFailureStop, OnFailureContinue:
		return true
	default:
		return false
	}
}

// AutomationAction is one step in an automation's ordered action chain.
// Mirrors BL-AT-01's `actions: [{type, params}, ...]` schema field-for-field.
type AutomationAction struct {
	StepType       StepType
	StepConfigJSON string
	// OnFailure defaults to OnFailureStop when empty, mirroring the proto
	// default — a chain whose later steps usually depend on an earlier one
	// having actually happened.
	OnFailure OnFailurePolicy
}
```

In `automation.go`, add `ProjectID string` and `Actions []AutomationAction`
fields to the `Automation` struct, and update `NewAutomation` (and
`ErrEmptyStepConfig`'s call sites) to add a new sentinel error and check:

```go
var ErrEmptyActions = apperrors.New(apperrors.KindInvalidArgument, "AUTOMATION_EMPTY_ACTIONS", "automation must have at least one action", nil)
```

```go
// inside NewAutomation, replacing/alongside the existing step-config check
if len(actions) == 0 {
	return Automation{}, ErrEmptyActions
}
for i := range actions {
	if actions[i].OnFailure == "" {
		actions[i].OnFailure = OnFailureStop
	} else if !actions[i].OnFailure.Valid() {
		return Automation{}, apperrors.New(apperrors.KindInvalidArgument, "AUTOMATION_INVALID_ON_FAILURE", "invalid on_failure policy", nil)
	}
}
```

Keep the existing `StepType`/`StepConfigJSON` fields and `ErrEmptyStepConfig`
in place for the deprecated single-step path; `NewAutomation` should accept
either a populated `Actions` list or (for back-compat callers not yet
updated) a single `StepType`/`StepConfigJSON` pair, normalizing the latter
into a one-element `Actions` list internally — check the actual current
`NewAutomation` signature/call sites in the file before editing so existing
callers keep compiling.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/automation-service/...
go test ./services/automation-service/internal/domain/...
```

Expected: clean build; add/run a table test asserting `len(actions) == 0` →
`ErrEmptyActions`, a single-action list succeeds, and unset `OnFailure`
defaults to `stop`.
