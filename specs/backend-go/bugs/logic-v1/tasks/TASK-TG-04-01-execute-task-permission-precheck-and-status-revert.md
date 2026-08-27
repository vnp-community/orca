# TASK-TG-04-01: `ExecuteTask` — permission pre-check + fix status-never-reverts-on-dispatch-failure bug

**From Solution:** SOL-TG-04
**Priority:** P1 — real correctness bug found while grounding this design: a task whose dev server is offline (or any dispatch failure) is marked `in_progress` PERMANENTLY on every `Execute` attempt, with no RPC to clear it
**Service:** `task-service`
**File:** `backend-go/services/task-service/internal/usecase/execute_task.go`
**Depends on:** none — this fix stands alone against the current `ExecuteTask` shape
**Status:** `[x]` DONE — ExecuteTask now runs the 'execute' permission pre-check + complexity determination BEFORE writing StatusInProgress, and reverts to the previous status on dispatch failure (no more permanently-stuck in_progress task). go test ./internal/usecase/... -run TestExecuteTask passes, including new revert-on-failure and permission-denied-never-writes-status regression tests.

---

## Context

`ExecuteTask.Execute` calls `repo.UpdateStatus(..., StatusInProgress)`
BEFORE determining complexity or attempting dispatch (`execute_task.go:57-59`),
and never reverts that status if the subsequent `simple.Execute`/
`complex.Execute` call fails (`execute_task.go:66-75`: the error is wrapped
and returned, but no compensating status write happens). Concretely: a task
whose project's dev server is offline gets marked `in_progress`
indefinitely on every failed `Execute` attempt, even though `SimpleExecutor`
correctly returns `TASK_EXECUTE_NO_CONNECTION` — the error surfaces to the
caller but the task is stuck. Separately, `ExecuteTask` never calls
`ResolvePermission` despite dispatching real work — every other mutating
RPC does per `task-service.md §3`.

This task fixes both, reordering `Execute` so the permission check and
complexity determination happen BEFORE the `in_progress` write, and adding
a revert-to-previous-status compensating write on dispatch failure. It does
NOT yet add worktree reuse-or-create or inline completion — that's
`TASK-TG-04-03`, layered on top of this fix.

## Changes to make

Rewrite `backend-go/services/task-service/internal/usecase/execute_task.go`:

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

type ExecuteTaskInput struct {
	TaskID    string
	RequestID string
}

type ExecuteTask struct {
	repo              TaskRepository
	edges             EdgeRepository
	simple            SimpleExecutor
	complex           ComplexExecutor
	resolvePermission *ResolvePermission
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

	// Pre-check 1: permission — every mutating RPC calls ResolvePermission
	// internally first per task-service.md §3; ExecuteTask never has,
	// despite dispatching real work. Closed here, BEFORE any status write.
	userID, _ := tenant.UserID(ctx)
	if _, err := uc.resolvePermission.Execute(ctx, ResolvePermissionInput{TaskID: in.TaskID, UserID: userID, Action: "execute"}); err != nil {
		return "", err // PermissionDenied, no status write happened yet
	}

	task, err := uc.repo.Get(ctx, tenantID, in.TaskID)
	if err != nil {
		return "", apperrors.New(apperrors.KindNotFound, "TASK_NOT_FOUND", "task not found", err)
	}
	previousStatus := task.Status

	complex, err := uc.isComplex(ctx, tenantID, in.TaskID) // unchanged — computed BEFORE any status write now, not after
	if err != nil {
		return "", apperrors.New(apperrors.KindInternal, "TASK_EXECUTE_EDGE_LOOKUP_FAILED", "failed to determine task complexity", err)
	}

	if err := uc.repo.UpdateStatus(ctx, tenantID, in.TaskID, domain.StatusInProgress); err != nil {
		return "", apperrors.New(apperrors.KindInternal, "TASK_EXECUTE_STATUS_UPDATE_FAILED", "failed to mark task in_progress", err)
	}

	var ref string
	if complex {
		ref, err = uc.complex.Execute(ctx, tenantID, in.TaskID, in.RequestID)
	} else {
		ref, err = uc.simple.Execute(ctx, tenantID, in.TaskID, in.RequestID)
	}
	if err != nil {
		// The fix: revert the in_progress write instead of leaving the task
		// stuck — a dispatch failure must never leave permanently-false
		// "running" state, since there is no other RPC to clear it.
		_ = uc.repo.UpdateStatus(ctx, tenantID, in.TaskID, previousStatus)
		return "", apperrors.New(apperrors.KindInternal, "TASK_EXECUTE_FAILED", "execution dispatch failed", err)
	}
	return ref, nil
}

func (uc *ExecuteTask) isComplex(ctx context.Context, tenantID, taskID string) (bool, error) {
	children, err := uc.edges.ListFrom(ctx, tenantID, taskID, domain.EdgeKindParentChild)
	if err != nil {
		return false, err
	}
	if len(children) > 0 {
		return true, nil
	}
	deps, err := uc.edges.ListFrom(ctx, tenantID, taskID, domain.EdgeKindDependsOn)
	if err != nil {
		return false, err
	}
	return len(deps) > 0, nil
}
```

Update `backend-go/services/task-service/cmd/server/main.go`'s
`executeTaskUC := usecase.NewExecuteTask(repo, repo, simpleExecutor,
complexExecutor)` call to also pass `resolvePermissionUC` (already
constructed earlier in the same function).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/task-service/...
go test ./services/task-service/internal/usecase/... -run TestExecuteTask -v
```

Expected: new tests — `simple.Execute`/`complex.Execute` returning an error
reverts status to the pre-dispatch value (fake `TaskRepository` asserts
`UpdateStatus` called twice: `InProgress` then the original status); a
`ResolvePermission` denial short-circuits before ANY `UpdateStatus` call at
all (regression guard against the false-`in_progress` bug this fixes).
Existing complexity-branch tests
(`TestExecuteTask_SimplePath_NoSubtasksNoDependencies`,
`TestExecuteTask_ComplexPath_HasSubtasks`, etc.) still pass — the branch
decision itself is unchanged, only reordered relative to the status write.
