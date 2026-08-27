# TASK-AT-03-02: New `domain/trigger.go` — `TriggerType`, `EventName`, `TriggerFilter`

**From Solution:** SOL-AT-03
**Priority:** P0
**Service:** `automation-service`
**File:** `backend-go/services/automation-service/internal/domain/trigger.go` (new), `backend-go/services/automation-service/internal/domain/automation.go`
**Depends on:** TASK-AT-03-01
**Status:** `[ ]` TODO

---

## Context

The domain layer needs a closed `EventName` set (the 5 documented events), a
closed comparison-grammar `TriggerFilter` (BR-AT-09, no `eval`/reflection),
and validation wiring into `NewAutomation`/`UpdateAutomation`.

## Changes to make

Create `backend-go/services/automation-service/internal/domain/trigger.go`:

```go
package domain

type TriggerType string

const (
	TriggerTypeCron   TriggerType = "cron"
	TriggerTypeManual TriggerType = "manual"
	TriggerTypeEvent  TriggerType = "event"
)

// EventName is a closed set — the five names BL-AT-03 documents. An
// unrecognized string is rejected by NewAutomation/UpdateAutomation, not
// silently accepted and never matched.
type EventName string

const (
	EventAgentCompleted  EventName = "agent:completed"
	EventAgentError      EventName = "agent:error"
	EventWorktreeCreated EventName = "worktree:created"
	EventPRMerged        EventName = "pr:merged"
	EventIssueAssigned   EventName = "issue:assigned"
)

func (e EventName) Valid() bool {
	switch e {
	case EventAgentCompleted, EventAgentError, EventWorktreeCreated, EventPRMerged, EventIssueAssigned:
		return true
	default:
		return false
	}
}

// TriggerFilter is BR-AT-09's closed comparison grammar — no arbitrary
// expression evaluation. {"field": "agent", "equals": "claude"}; field is a
// dot-path into the event payload, "equals" is the only operator v1 needs.
type TriggerFilter struct {
	Field  string
	Equals string
}

// Matches performs a fail-safe-false dot-path lookup + string-equals — an
// automation with a broken filter never fires rather than firing on
// everything.
func (f TriggerFilter) Matches(payload map[string]any) bool {
	parts := strings.Split(f.Field, ".")
	var cur any = payload
	for _, p := range parts {
		m, ok := cur.(map[string]any)
		if !ok {
			return false
		}
		cur, ok = m[p]
		if !ok {
			return false
		}
	}
	s, ok := cur.(string)
	return ok && s == f.Equals
}
```

In `automation.go`, add `TriggerType TriggerType`, `TriggerEvent EventName`,
`TriggerFilter *TriggerFilter` to `Automation`. In `NewAutomation` (and
`UpdateAutomation` if validation is duplicated there), add:

```go
if trigger == "" {
	trigger = TriggerTypeCron // back-compat default
}
if trigger == TriggerTypeEvent {
	if !event.Valid() {
		return Automation{}, apperrors.New(apperrors.KindInvalidArgument, "AUTOMATION_INVALID_TRIGGER_EVENT", "trigger_event must be one of the 5 documented event names", nil)
	}
} else if event != "" {
	return Automation{}, apperrors.New(apperrors.KindInvalidArgument, "AUTOMATION_UNEXPECTED_TRIGGER_EVENT", "trigger_event must be empty unless trigger_type=event", nil)
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/automation-service/...
go test ./services/automation-service/internal/domain/...
```

Expected: `TriggerFilter.Matches` — dot-path lookup against a nested
payload, missing field → `false`; `TriggerType == Event` requires a valid
`TriggerEvent`; `TriggerType != Event` rejects a non-empty `TriggerEvent`.
Add these cases to `trigger_test.go` (new) and `automation_test.go`.
