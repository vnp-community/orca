# TASK-AT-02-01: 2-hour outer run timeout on `RunNow` (BR-AT-06)

**From Solution:** SOL-AT-02
**Priority:** P0
**Service:** `automation-service`
**File:** `backend-go/services/automation-service/internal/usecase/run_now.go`
**Depends on:** none (assumes SOL-AT-01's action-loop shape from TASK-AT-01-05 already exists; if not yet merged, apply the timeout wrap around the existing single `ExecuteAdHocStep` call instead)
**Status:** `[x]` DONE — run_now.go wraps the action loop in context.WithTimeout(runTimeout=2h, overridable via uc.timeout); AUTOMATION_RUN_TIMEOUT (new apperrors.KindDeadlineExceeded) on expiry.

---

## Context

`RunNow` has no outer deadline bounding the whole dispatched run — only
`workflow-service`'s own per-step 30-minute deadline exists. BR-AT-06 needs
a 2-hour budget around the entire action loop, distinguishable in run
history from an ordinary step failure.

## Changes to make

In `run_now.go`, add a package-level constant and wrap the action loop in a
`context.WithTimeout`:

```go
const runTimeout = 2 * time.Hour // BR-AT-06, automation-service's own deadline-override convention
```

```go
func (uc *RunNow) Execute(ctx context.Context, in RunNowInput) (domain.AutomationRun, error) {
	// ... existing tenant/idempotency/pending-run setup unchanged ...
	runCtx, cancel := context.WithTimeout(ctx, runTimeout)
	defer cancel()
	for i, action := range automation.Actions {
		result, execErr := uc.executor.ExecuteAdHocStep(runCtx, ExecuteAdHocStepInput{ /* ... */ })
		if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
			failed, _ := running.MarkFailed(time.Now().UTC(), "automation run exceeded 2h timeout")
			_ = uc.runs.UpdateStatus(ctx, failed) // ctx, not runCtx — runCtx is already expired
			return failed, apperrors.New(apperrors.KindDeadlineExceeded, "AUTOMATION_RUN_TIMEOUT", "run exceeded 2h timeout", runCtx.Err())
		}
		// ... existing per-action success/failure handling ...
	}
	// ...
}
```

Make `runTimeout` overridable (package-level `var` or a constructor
parameter on `RunNow`) so tests can shorten it rather than waiting 2 real
hours.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/automation-service/...
go test ./services/automation-service/internal/usecase/... -run TestRunNow
```

Expected: a fake `WorkflowStepExecutor` that blocks on `ctx.Done()` →
`RunNow.Execute` returns `AUTOMATION_RUN_TIMEOUT` within the test's
shortened `runTimeout` override; run row lands `failed` with the timeout
reason. Add this case to `run_now_test.go`.
