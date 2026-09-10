# BE-SOL-005: Permission precheck, status revert, real `ComplexExecutor`, richer prompt, correct env injection

**Resolves:** [CR-TG-005](../../../../../../docs/crs/v4/task-graph/CR-TG-005-task-agent-execution-permission-and-complex-executor.md)
**Service:** `task-service` (+ calls into `orchestration-service`'s new RPCs) — **no `agent/` change required**, see §1
**Depends on:** [BE-SOL-001](./BE-SOL-001-orcatask-data-model-widening.md) (prompt-building fields), [BE-SOL-003](./BE-SOL-003-task-access-control-team-scope-and-sharing.md) (permission precheck calls `ResolvePermission`), [BE-SOL-004](./BE-SOL-004-orchestration-service-coordinator-run-lifecycle.md) (`StartCoordinatorRun` must exist before `ComplexExecutor` can call it)
**Affected files (proposed):**
- `backend-go/services/task-service/internal/usecase/execute_task.go` (permission precheck, status revert)
- `backend-go/services/task-service/internal/adapter/grpcclient/complex_executor.go` (replace `StubComplexExecutor`)
- `backend-go/services/task-service/internal/adapter/grpcclient/simple_executor.go` (`Env` field, `buildExecutePrompt` enrichment)
- `backend-go/proto/orca/task/v1/task.proto` (`ReportTaskExecutionResult` service-to-service RPC)
**Status:** 📋 Proposed — not yet implemented

---

## 1. Current state — two things the CR assumed missing already exist

Reading `simple_executor.go`/`execute_task.go` in full (not just the CR's
cited fragments) surfaces two corrections that shrink this solution's real
scope:

1. **`agent.execPrompt` is already the correct relay method** — confirmed,
   the file's own extensive doc comment documents the exact investigation
   (`agentExecMethod`'s history, TASK-224 "Gap 1, closed") that established
   this. No change needed here — the CR was already right not to touch it.
2. **The "Run-Agent prompt override" gap CR-TG-007 (frontend) describes is
   already closed on the backend-go side** — `ExecuteTaskInput.Prompt`
   (`execute_task.go:16-18`, citing `docs/backlog/BACKLOG-016`),
   `TaskServiceExecuteRequest.prompt` (read via `req.GetPrompt()` in
   `server.go`), and `SimpleExecutor.Execute(ctx, tenantID, taskID,
   requestID, prompt string)`'s `effectivePrompt := prompt; if
   effectivePrompt == "" { effectivePrompt = buildExecutePrompt(task) }`
   (`simple_executor.go:159-163`) form a complete, already-shipped
   override path. **This solution does not add a prompt field — it
   already exists.** The actual remaining gap is purely on the frontend
   side (the RPC call site never sends it) — see
   [FE-SOL-001](../../../../../frontend/crs/v4/task-graph/solutions/FE-SOL-001-task-crud-board-grant-ui.md),
   which is corrected accordingly.

**No `agent/` code change is needed for the env-injection fix in §4** either
— confirmed by reading `agent-print-mode-exec.ts` in full: `params.env` is
already accepted and merged "on top of `buildAgentEnv()`'s base env" (the
file's own doc comment, `agent-print-mode-exec.ts:47-50`), meaning a
backend-go-supplied `env.ORCA_TASK_ID` already overrides the auto-derived
(and wrong) `taskId: stepId ?? ''` value `buildAgentEnv` uses internally
(`agent-print-mode-exec.ts:109-116`). See
[SOL-AG-TG-001](../../../../../agent/crs/v4/task-graph/solutions/SOL-AG-TG-001-agent-env-contract-assessment.md)
for the full assessment confirming zero agent-side code is required.

## Design rationale (grounded in real code)

`execute_task.go`'s own doc comment is unusually candid about exactly what
this solution needs to fix: *"Both executors are STUBS in this scaffold...
only the branch decision itself is real,"* and separately: *"task-service
has no execution-completion callback at all, so nothing ever transitions a
task back out of `in_progress` today."* This solution closes both
admissions directly, plus the permission-precheck gap the CR identified
(confirmed: `Execute()` calls `tenant.RequireTenantID`, `repo.UpdateStatus`,
`isComplex`, and the two executors — never `ResolvePermission`).

## Design — permission precheck + status revert

```go
// execute_task.go
func (uc *ExecuteTask) Execute(ctx context.Context, in ExecuteTaskInput) (string, error) {
    tenantID, err := tenant.RequireTenantID(ctx)
    if err != nil { return "", apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err) }

    level, err := uc.resolvePermission.Execute(ctx, ResolvePermissionInput{TaskID: in.TaskID, UserID: in.UserID, Action: "execute"})
    if err != nil { return "", apperrors.New(apperrors.KindPermissionDenied, "TASK_EXECUTE_DENIED", "caller cannot execute this task", err) }
    _ = level // ResolvePermission itself returns apperrors.KindPermissionDenied when OPA denies — see BE-SOL-003

    if err := uc.repo.UpdateStatus(ctx, tenantID, in.TaskID, domain.StatusInProgress); err != nil {
        return "", apperrors.New(apperrors.KindInternal, "TASK_EXECUTE_STATUS_UPDATE_FAILED", "failed to mark task in_progress", err)
    }

    complex, err := uc.isComplex(ctx, tenantID, in.TaskID)
    if err != nil { return "", apperrors.New(apperrors.KindInternal, "TASK_EXECUTE_EDGE_LOOKUP_FAILED", "failed to determine task complexity", err) }

    var ref string
    if complex {
        ref, err = uc.complex.Execute(ctx, tenantID, in.TaskID, in.RequestID, in.Prompt)
    } else {
        ref, err = uc.simple.Execute(ctx, tenantID, in.TaskID, in.RequestID, in.Prompt)
    }
    if err != nil {
        // NEW — compensating write, closes the "stuck in_progress forever" gap
        if revertErr := uc.repo.UpdateStatus(ctx, tenantID, in.TaskID, domain.StatusBlocked); revertErr != nil {
            log.Error("execute_task: failed to revert status after dispatch failure", "taskID", in.TaskID, "err", revertErr)
        }
        return "", apperrors.New(apperrors.KindInternal, "TASK_EXECUTE_FAILED", "execution dispatch failed, task reverted to blocked", err)
    }
    return ref, nil
}
```

`ExecuteTaskInput` gains a `UserID string` field (`TaskServiceExecuteRequest`
needs the same addition) — following the existing convention
`ResolvePermissionRequest.UserID` already uses (task-service passes caller
identity explicitly on the wire, no auth-context extractor exists today,
per [BE-SOL-003](./BE-SOL-003-task-access-control-team-scope-and-sharing.md)'s
identical note for `CreateTaskRequest.CreatorID`).

## Design — real `ComplexExecutor`

```go
// complex_executor.go — replaces StubComplexExecutor
type ComplexExecutor struct {
    orchestration orchestrationv1.OrchestrationServiceClient // BE-SOL-004's StartCoordinatorRun
}

func (e *ComplexExecutor) Execute(ctx context.Context, tenantID, taskID, requestID, prompt string) (string, error) {
    run, err := e.orchestration.StartCoordinatorRun(ctx, &orchestrationv1.StartCoordinatorRunRequest{
        OriginTaskId: taskID, CoordinatorHandle: fmt.Sprintf("task-%s", taskID),
    })
    if err != nil { return "", fmt.Errorf("complex_executor: start_coordinator_run: %w", err) }
    return run.GetId(), nil
}
```

## Design — `ReportTaskExecutionResult` callback

```protobuf
// task.proto — service-to-service RPC, orchestration-service calls this
// when a CoordinatorRun it started reaches a terminal state.
rpc ReportTaskExecutionResult(ReportTaskExecutionResultRequest) returns (google.protobuf.Empty);
```

This is the same RPC [`docs/crs/v3/flow-task/CR-FLOW-TASK-002`](../../../../../../docs/crs/v3/flow-task/CR-FLOW-TASK-002-workflow-as-task-execution-engine.md)
(§4) already designs for Engine 3 (Workflow) — this solution is Engine 2's
caller of the **same** RPC, with a `source: "orchestration"` field so the
usecase can distinguish log origin, per that CR's own note: *"if SOL-TG-04
has added this RPC for Engine 2, Engine 3 TÁI SỬ DỤNG cùng RPC — request
thêm field `engine`... KHÔNG tạo RPC riêng."* Whichever of BE-SOL-005 or
CR-FLOW-TASK-002 lands first should add the RPC; the other adopts it as-is.

## Design — env injection (backend-go side only, per §1)

```go
// simple_executor.go
type agentExecPromptParams struct {
    Prompt       string            `json:"prompt"`
    WorktreePath string            `json:"worktreePath"`
    StepID       string            `json:"stepId,omitempty"`
    Env          map[string]string `json:"env,omitempty"` // NEW
}

// Execute() — set explicitly, do not rely on StepID being mistaken for a task id
paramsJSON, err := json.Marshal(agentExecPromptParams{
    Prompt: effectivePrompt, WorktreePath: worktreePath, StepID: requestID,
    Env: map[string]string{"ORCA_TASK_ID": taskID, "ORCA_PROJECT_ID": task.ProjectID},
})
```

## Design — richer `buildExecutePrompt` (depends on BE-SOL-001/002 fields)

```go
// simple_executor.go
func buildExecutePrompt(task domain.Task, deps []domain.Task) string {
    var b strings.Builder
    fmt.Fprintf(&b, "Complete the following task.\n\nTask: %s\n", task.Title)
    if task.Description != "" { fmt.Fprintf(&b, "\n%s\n", task.Description) }
    if task.AIContext != "" { fmt.Fprintf(&b, "\nContext:\n%s\n", task.AIContext) }
    if len(deps) > 0 { fmt.Fprintf(&b, "\nAlready completed (dependencies):\n%s\n", summarize(deps)) }
    if task.PromptTemplate != "" { fmt.Fprintf(&b, "\nSpecific instructions:\n%s\n", task.PromptTemplate) }
    return b.String()
}
```

Kept as plain text, per the file's own documented convention ("this
codebase's one existing AI-task-execution prompt-building convention...
plain text... because there is no live agent contract yet to confirm a
structured one against") — not switched to JSON or any new format.

## Test plan

- `Execute` with a caller lacking `execute`-level permission → denied,
  task status unchanged (not even transiently `in_progress`).
- Dispatch failure (simulate `SimpleExecutor`/`ComplexExecutor` returning an
  error) → task status ends at `blocked`, not stuck `in_progress`.
- `ComplexExecutor.Execute` calls `StartCoordinatorRun` with the right
  `OriginTaskId`; no more `stub-orchestration-exec:` string anywhere in the
  codebase (grep assertion in CI).
- `agentExecPromptParams.Env` contains the real `taskID` (not `requestID`)
  — integration test against a fake agent relay asserting the JSON payload.
- `buildExecutePrompt` snapshot test once BE-SOL-001/002 fields are wired.

## Not in scope (per the CR)

- Continuous PTY/stdout streaming — [BE-SOL-006](./BE-SOL-006-task-execute-streaming-relay.md).
- Adding an override-prompt field — already shipped, see §1.
- Any `agent/` code change — none required, see §1 and
  [SOL-AG-TG-001](../../../../../agent/crs/v4/task-graph/solutions/SOL-AG-TG-001-agent-env-contract-assessment.md).
- Worktree reuse-or-create logic beyond what `ProjectExecutionResolver`
  already does — not touched, `ResolveConnection`'s existing behavior is
  unchanged by this solution.

## References

- [CR-TG-005](../../../../../../docs/crs/v4/task-graph/CR-TG-005-task-agent-execution-permission-and-complex-executor.md)
- [SOL-TG-04](../../../../bugs/logic-v1/solutions/SOL-TG-04-task-agent-execution.md)
- `backend-go/services/task-service/internal/usecase/execute_task.go`
- `backend-go/services/task-service/internal/adapter/grpcclient/simple_executor.go`
- `agent/src/relay/agent-print-mode-exec.ts`
