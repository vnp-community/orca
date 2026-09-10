# TASK-AT-03-06: Cycle detection over event-triggered automations (BR-AT-10 / BR-AT-04)

**From Solution:** SOL-AT-03
**Priority:** P1
**Service:** `automation-service`
**File:** `backend-go/services/automation-service/internal/usecase/create_automation.go`, `backend-go/services/automation-service/internal/usecase/update_automation.go`, `backend-go/services/automation-service/internal/usecase/trigger_cycle.go` (new)
**Depends on:** TASK-AT-03-02
**Status:** `[x]` DONE — usecase/trigger_cycle.go: DetectTriggerCycle (Kahn's-algorithm findCycle) + ListEventTriggered; wired into Create/UpdateAutomation for trigger_type=event; actionToEvents limited to the real closed StepType set (documented deviation from the task's illustrative create_worktree/create_pr step types, which don't exist in this service's StepType enum).

---

## Context

Modeled on `workflow-service`'s `BuildWaves`/`ErrCyclicDependency`
precedent. At create/update time for an event-triggered automation, build a
directed graph over the tenant's event-triggered automations (action → event
it could emit → next automation's trigger) and reject a cycle. This subsumes
SOL-AT-01's placeholder self-reference guard — a same-automation, single-hop
cycle is the `X == Y` degenerate case of this same graph.

## Changes to make

Create `backend-go/services/automation-service/internal/usecase/trigger_cycle.go`:

```go
package usecase

// actionToEvents is the fixed, closed mapping from an action's StepType to
// the event(s) it could emit — mirrors EventName's closed set.
var actionToEvents = map[domain.StepType][]domain.EventName{
	domain.StepTypeAgent:          {domain.EventAgentCompleted, domain.EventAgentError},
	domain.StepTypeCreateWorktree: {domain.EventWorktreeCreated},
	domain.StepTypeCreatePR:       {domain.EventPRMerged},
	// commit/notify/cleanup emit none of the 5 documented events — no edge.
}

// DetectTriggerCycle builds a directed graph over tenantID's event-triggered
// automations (including candidate) and returns ErrTriggerCycle naming the
// offending automation IDs if one exists.
func DetectTriggerCycle(ctx context.Context, repo AutomationRepository, tenantID string, candidate domain.Automation) error {
	all, err := repo.ListEventTriggered(ctx, tenantID) // new port method, see below
	if err != nil {
		return err
	}
	nodes := replaceOrAppend(all, candidate) // candidate overrides its own prior version on update

	graph := map[string][]string{} // automation ID -> IDs it can trigger
	byEvent := map[domain.EventName][]string{}
	for _, a := range nodes {
		if a.TriggerType == domain.TriggerTypeEvent {
			byEvent[a.TriggerEvent] = append(byEvent[a.TriggerEvent], a.ID)
		}
	}
	for _, a := range nodes {
		for _, action := range a.Actions {
			for _, ev := range actionToEvents[action.StepType] {
				graph[a.ID] = append(graph[a.ID], byEvent[ev]...)
			}
		}
	}

	if cycle := findCycle(graph); cycle != nil { // Kahn's algorithm, in-degree/BFS
		return apperrors.New(apperrors.KindFailedPrecondition, "AUTOMATION_TRIGGER_CYCLE",
			fmt.Sprintf("cyclic automation trigger chain: %v", cycle), nil)
	}
	return nil
}
```

Add `ListEventTriggered(ctx, tenantID) ([]domain.Automation, error)` to
`AutomationRepository` port and implement it in `repository.go` (`WHERE
tenant_id = $1 AND trigger_type = 'event'`).

Call `DetectTriggerCycle` from `create_automation.go`/`update_automation.go`
before persisting, only when `candidate.TriggerType ==
domain.TriggerTypeEvent`.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/automation-service/...
go test ./services/automation-service/internal/usecase/... -run TestCreateAutomation
go test ./services/automation-service/internal/usecase/... -run TestTriggerCycle
```

Expected: automation A (trigger=`agent:completed`, action=`create_pr`) +
automation B (trigger=`pr:merged`, action=`run_agent`) — creating B after A
exists → rejected `AUTOMATION_TRIGGER_CYCLE`; creating either alone
succeeds; a same-automation self-reference (X == Y degenerate case) is also
caught.
