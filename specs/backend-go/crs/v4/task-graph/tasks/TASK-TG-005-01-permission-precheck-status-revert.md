# TASK-TG-005-01: `ExecuteTask` permission precheck + status-revert-on-dispatch-failure

**From Solution:** BE-SOL-005
**Priority:** P0
**Service:** `task-service`
**File:** `backend-go/services/task-service/internal/usecase/execute_task.go`, `backend-go/proto/orca/task/v1/task.proto` (`TaskServiceExecuteRequest.user_id`), `backend-go/services/task-service/internal/adapter/grpc/server.go` (`Execute` handler)
**Depends on:** TASK-TG-003 tasks (the `ResolvePermission` usecase this task calls — already exists today per Context below, but TASK-TG-003-06's `action` wire fix and TASK-TG-003-01's real `TeamScopeResolver` should land first so the precheck this task adds is actually meaningful, not silently passing every caller via the stub)
**Status:** `[ ]` TODO

---

## Context

Verified directly (`internal/usecase/execute_task.go:1-99`, read in full):
`Execute` (lines 53-81) today calls `tenant.RequireTenantID`,
`uc.repo.UpdateStatus(..., StatusInProgress)` (line 62 — unconditional,
BEFORE any complexity/dispatch decision), `uc.isComplex`, then one of the
two executors — **never `ResolvePermission`**. The file's own doc comment
(lines 30-41) is candid about the second gap this task closes: *"task-service
has no execution-completion callback at all, so nothing ever transitions a
task back out of `in_progress` today."* A dispatch failure today leaves the
task stuck `in_progress` forever with no compensating write.

`usecase.ResolvePermission` already exists and is fully implemented
(`internal/usecase/resolve_permission.go:39-91`, TASK-TG-003 series) — this
task is purely `ExecuteTask` calling it, not building it.

`ExecuteTaskInput` (`execute_task.go:11-19`) has `{TaskID, RequestID,
Prompt}` — no `UserID` field today. `TaskServiceExecuteRequest`
(`task.proto`, confirmed 3 fields: `task_id=1, request_id=2, prompt=3`)
needs the same addition at field `4`, following
`ResolvePermissionRequest.UserID`'s existing convention of passing caller
identity explicitly on the wire (this service has no auth-context
user-id extractor — confirmed, 0 hits for a `RequireUserID`-style helper in
`common/`).

## Changes to make

**1. `task.proto`** — `TaskServiceExecuteRequest` gains `string user_id =
4;`.

**2. `internal/usecase/execute_task.go`** — add `UserID` to the input,
reorder `Execute` to precheck permission BEFORE the status write, and add
the compensating revert:

```go
type ExecuteTaskInput struct {
	TaskID    string
	RequestID string
	UserID    string // NEW
	Prompt    string
}

type ExecuteTask struct {
	repo              TaskRepository
	edges             EdgeRepository
	simple            SimpleExecutor
	complex           ComplexExecutor
	resolvePermission *ResolvePermission // NEW
}

func NewExecuteTask(repo TaskRepository, edges EdgeRepository, simple SimpleExecutor, complex ComplexExecutor, resolvePermission *ResolvePermission) *ExecuteTask {
	return &ExecuteTask{repo: repo, edges: edges, simple: simple, complex: complex, resolvePermission: resolvePermission}
}

func (uc *ExecuteTask) Execute(ctx context.Context, in ExecuteTaskInput) (string, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return "", apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}
	if in.TaskID == "" {
		return "", apperrors.New(apperrors.KindInvalidArgument, "TASK_EXECUTE_INVALID", "task_id is required", nil)
	}

	// NEW — permission precheck, BEFORE any status write.
	if _, err := uc.resolvePermission.Execute(ctx, ResolvePermissionInput{TaskID: in.TaskID, UserID: in.UserID, Action: "execute"}); err != nil {
		return "", apperrors.New(apperrors.KindPermissionDenied, "TASK_EXECUTE_DENIED", "caller cannot execute this task", err)
	}

	if err := uc.repo.UpdateStatus(ctx, tenantID, in.TaskID, domain.StatusInProgress); err != nil {
		return "", apperrors.New(apperrors.KindInternal, "TASK_EXECUTE_STATUS_UPDATE_FAILED", "failed to mark task in_progress", err)
	}

	complex, err := uc.isComplex(ctx, tenantID, in.TaskID)
	if err != nil {
		return "", apperrors.New(apperrors.KindInternal, "TASK_EXECUTE_EDGE_LOOKUP_FAILED", "failed to determine task complexity", err)
	}

	var ref string
	if complex {
		ref, err = uc.complex.Execute(ctx, tenantID, in.TaskID, in.RequestID, in.Prompt)
	} else {
		ref, err = uc.simple.Execute(ctx, tenantID, in.TaskID, in.RequestID, in.Prompt)
	}
	if err != nil {
		// NEW — compensating write, closes the "stuck in_progress forever" gap.
		if revertErr := uc.repo.UpdateStatus(ctx, tenantID, in.TaskID, domain.StatusBlocked); revertErr != nil {
			log.Error("execute_task: failed to revert status after dispatch failure", "taskID", in.TaskID, "err", revertErr)
		}
		return "", apperrors.New(apperrors.KindInternal, "TASK_EXECUTE_FAILED", "execution dispatch failed, task reverted to blocked", err)
	}
	return ref, nil
}
```

`domain.StatusBlocked` requires TASK-TG-001-02 to have landed
(`domain.Status`/the 8-value enum) — if this task lands before
TASK-TG-001-02, the revert target has no valid constant to use yet; do not
substitute a different existing status (e.g. `StatusOpen`) as a stopgap,
since "reverted because dispatch failed" and "never started" are distinct,
observable states a caller/UI needs to tell apart.

**3. `internal/adapter/grpc/server.go`** — `Execute` handler passes
`req.GetUserId()` through to `ExecuteTaskInput.UserID`.

**4. `cmd/server/main.go`** — `NewExecuteTask`'s wiring gains the
`*usecase.ResolvePermission` instance (already constructed for the
`ResolvePermission` RPC's own wiring — reuse the same instance, don't
construct a second one).

## Test plan

- `Execute` with a caller lacking `execute`-level permission → denied, task
  status unchanged (not even transiently `in_progress` — this is why the
  precheck must run BEFORE the status write, not after).
- Dispatch failure (simulate `SimpleExecutor`/`ComplexExecutor` returning an
  error) → task status ends at `StatusBlocked`, not stuck `StatusInProgress`.
- A revert-write failure itself (simulated) is logged but does not mask the
  original dispatch error returned to the caller.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/task-service/...
go test ./services/task-service/internal/usecase/... -run TestExecuteTask -v
```

Expected: clean build; permission-denied test asserts zero calls reach
`uc.repo.UpdateStatus` at all (use a fake `TaskRepository` that fails the
test if `UpdateStatus` is invoked); dispatch-failure test asserts the final
persisted status is `blocked`, via a fake repository recording every
`UpdateStatus` call in order (`in_progress` then `blocked`).
