# TASK-AT-01-05: `RunNow` dispatches the ordered `Actions` chain instead of a single step

**From Solution:** SOL-AT-01
**Priority:** P1
**Service:** `automation-service`
**File:** `backend-go/services/automation-service/internal/usecase/run_now.go`, `backend-go/services/automation-service/internal/domain/automation_run.go`
**Depends on:** TASK-AT-01-02
**Status:** `[x]` DONE — automation_run.go gained ActionResult/ActionResults; run_now.go loops automation.Actions honoring OnFailure, per-action request_id suffix; run_now_test.go covers stop/continue.

---

## Context

`RunNow.Execute`'s single `uc.executor.ExecuteAdHocStep(...)` call becomes a
loop over `automation.Actions`, honoring each action's `OnFailure` policy.
This is the shared surface SOL-AT-02 later wraps with a timeout — this task
only introduces the loop shape and the per-action result schema.

## Changes to make

In `automation_run.go`, add a per-action result type and field to
`AutomationRun`:

```go
type ActionResult struct {
	Index        int
	Status       string // "succeeded" | "failed"
	OutputJSON   string
	ErrorMessage string
}
```

Add `ActionResults []ActionResult` to `AutomationRun`, keeping the existing
`OutputJSON`/`ErrorMessage` fields (now holding the *last* action's output,
for backward wire-compatibility with any caller still reading them
directly).

In `run_now.go`, replace the single `ExecuteAdHocStep` call with a loop:

```go
var results []domain.ActionResult
runFailed := false
for i, action := range automation.Actions {
	result, execErr := uc.executor.ExecuteAdHocStep(ctx, ExecuteAdHocStepInput{
		TenantID:       tenantID,
		StepType:       action.StepType,
		StepConfigJSON: action.StepConfigJSON,
		RequestID:      fmt.Sprintf("%s:%d", in.RequestID, i), // per-action idempotency suffix
	})
	ar := domain.ActionResult{Index: i}
	switch {
	case execErr != nil:
		ar.Status = "failed"
		ar.ErrorMessage = execErr.Error()
	case result.Status == "failed":
		ar.Status = "failed"
		ar.ErrorMessage = result.ErrorMessage
		ar.OutputJSON = result.OutputJSON
	default:
		ar.Status = "succeeded"
		ar.OutputJSON = result.OutputJSON
	}
	results = append(results, ar)
	if ar.Status == "failed" {
		policy := action.OnFailure
		if policy == "" {
			policy = domain.OnFailureStop
		}
		if policy == domain.OnFailureStop {
			runFailed = true
			break
		}
	}
}
```

Wire `results` into the run's `ActionResults` field and the existing
success/failure status transition logic — check the current status-setting
code around the removed single-call site (`MarkSucceeded`/`MarkFailed`) to
preserve its existing shape, using `runFailed` (or "last action's status if
never explicitly stopped") to decide the terminal call.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/automation-service/...
go test ./services/automation-service/internal/usecase/... -run TestRunNow
```

Expected: a 3-action chain where action 2 fails with `OnFailureStop` → run
marked failed, action 3 never dispatched (assert fake executor call count ==
2); same chain with `OnFailureContinue` on action 2 → all 3 actions
dispatched, `ActionResults` records action 2's failure. Add these cases to
`run_now_test.go` if not already present.
