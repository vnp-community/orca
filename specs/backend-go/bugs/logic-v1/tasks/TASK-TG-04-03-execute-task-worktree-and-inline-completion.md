# TASK-TG-04-03: `ExecuteTask` — worktree reuse-or-create + inline completion for the simple path

**From Solution:** SOL-TG-04
**Priority:** P1
**Service:** `task-service`
**File:** `backend-go/services/task-service/internal/usecase/execute_task.go`
**Depends on:** TASK-TG-04-01, TASK-TG-04-02, TASK-TG-01-04 (`actual_hours`/`worktree_id` columns, `CompleteExecution` repo method)
**Status:** [x] DONE — ExecuteTask now resolves dev-server connectivity + worktree reuse-or-create (via WorktreeProvisioner) before the in_progress write, and completes the simple path INLINE (StatusReview + actual_hours via CompleteExecution) in the same call; the complex path returns ExecuteResult{Async:true} with no inline completion. TaskServiceExecuteResponse gained an `async` proto field; Repository.CompleteExecution implemented. `go test ./services/task-service/internal/usecase/... -run TestExecuteTask` passes (16/16, including new worktree-reuse/-create and inline-completion regression tests); `go build` clean across every backend-go service.

---

## Context

**Key finding from reading the current code**: the simple path does NOT
need an async completion callback at all. `SimpleExecutor.Execute` already
blocks synchronously until the Dev Server Agent's `agent.execPrompt` call
returns (`simple_executor.go:92-98`'s own doc comment). That means
`ExecuteTask` already KNOWS the simple path finished, successfully or not,
before its own `Execute` call returns to the caller — the "no completion
callback" gap is real for the *complex* path (async coordinator dispatch,
`TASK-TG-04-05`) but is a design choice already available and unused for
the *simple* path. This task closes it inline, with no new RPC, on top of
`TASK-TG-04-01`'s bug fix.

## Changes to make

Extend `backend-go/services/task-service/internal/usecase/execute_task.go`'s
`ExecuteTask` (from `TASK-TG-04-01`) with a `worktrees WorktreeProvisioner`
field, a `clock` for `actual_hours`, and the worktree/inline-completion
steps:

```go
type ExecuteTask struct {
	repo              TaskRepository
	edges             EdgeRepository
	simple            SimpleExecutor
	complex           ComplexExecutor
	resolvePermission *ResolvePermission
	worktrees         WorktreeProvisioner
	resolver          ProjectExecutionResolver
	clock             Clock // Now() time.Time — see TASK-TG-03-06's note on adding a shared Clock port
}

func NewExecuteTask(repo TaskRepository, edges EdgeRepository, simple SimpleExecutor, complex ComplexExecutor, resolvePermission *ResolvePermission, worktrees WorktreeProvisioner, resolver ProjectExecutionResolver, clock Clock) *ExecuteTask {
	return &ExecuteTask{repo: repo, edges: edges, simple: simple, complex: complex, resolvePermission: resolvePermission, worktrees: worktrees, resolver: resolver, clock: clock}
}

func (uc *ExecuteTask) Execute(ctx context.Context, in ExecuteTaskInput) (ExecuteResult, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return ExecuteResult{}, apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}
	if in.TaskID == "" {
		return ExecuteResult{}, apperrors.New(apperrors.KindInvalidArgument, "TASK_EXECUTE_INVALID", "task_id is required", nil)
	}

	userID, _ := tenant.UserID(ctx)
	if _, err := uc.resolvePermission.Execute(ctx, ResolvePermissionInput{TaskID: in.TaskID, UserID: userID, Action: "execute"}); err != nil {
		return ExecuteResult{}, err
	}

	task, err := uc.repo.Get(ctx, tenantID, in.TaskID)
	if err != nil {
		return ExecuteResult{}, apperrors.New(apperrors.KindNotFound, "TASK_NOT_FOUND", "task not found", err)
	}
	previousStatus := task.Status

	complex, err := uc.isComplex(ctx, tenantID, in.TaskID)
	if err != nil {
		return ExecuteResult{}, apperrors.New(apperrors.KindInternal, "TASK_EXECUTE_EDGE_LOOKUP_FAILED", "failed to determine task complexity", err)
	}

	// Pre-check 2: dev-server-online — resolved once here, BEFORE the
	// in_progress write, so a disconnected project fails before any status
	// mutation at all.
	connectionID, resolvedPath, connected, err := uc.resolver.ResolveConnection(ctx, tenantID, task.ProjectID)
	if err != nil || !connected {
		return ExecuteResult{}, apperrors.New(apperrors.KindFailedPrecondition, "TASK_EXECUTE_NO_CONNECTION", "task's project has no connected dev server", err)
	}

	// Worktree reuse-or-create.
	worktreeID, worktreePath, err := uc.worktrees.EnsureWorktree(ctx, tenantID, task)
	if err != nil {
		return ExecuteResult{}, apperrors.New(apperrors.KindInternal, "TASK_EXECUTE_WORKTREE_FAILED", "failed to provision worktree", err)
	}
	if worktreePath == "" {
		worktreePath = resolvedPath // reuse path: EnsureWorktree returns "" for path on reuse, see TASK-TG-04-02
	}
	if worktreeID != task.WorktreeID {
		if err := uc.repo.UpdateWorktreeID(ctx, tenantID, in.TaskID, worktreeID); err != nil {
			return ExecuteResult{}, apperrors.New(apperrors.KindInternal, "TASK_EXECUTE_WORKTREE_PERSIST_FAILED", "failed to persist worktree id", err)
		}
	}

	if err := uc.repo.UpdateStatus(ctx, tenantID, in.TaskID, domain.StatusInProgress); err != nil {
		return ExecuteResult{}, apperrors.New(apperrors.KindInternal, "TASK_EXECUTE_STATUS_UPDATE_FAILED", "failed to mark task in_progress", err)
	}
	dispatchStart := uc.clock.Now()

	if complex {
		ref, err := uc.complex.Execute(ctx, tenantID, in.TaskID, in.RequestID)
		if err != nil {
			_ = uc.repo.UpdateStatus(ctx, tenantID, in.TaskID, previousStatus)
			return ExecuteResult{}, apperrors.New(apperrors.KindInternal, "TASK_EXECUTE_FAILED", "execution dispatch failed", err)
		}
		// No further status write here — StatusReview/Done arrives later
		// via ReportTaskExecutionResult (TASK-TG-04-05).
		return ExecuteResult{ExecutionRef: ref, Async: true}, nil
	}

	// Simple path: SimpleExecutor.Execute blocks until the CLI process
	// exits — the completion transition happens INLINE, same call.
	result, err := uc.simple.Execute(ctx, tenantID, in.TaskID, in.RequestID)
	if err != nil {
		_ = uc.repo.UpdateStatus(ctx, tenantID, in.TaskID, previousStatus)
		return ExecuteResult{}, apperrors.New(apperrors.KindInternal, "TASK_EXECUTE_FAILED", "execution dispatch failed", err)
	}
	actualHours := uc.clock.Now().Sub(dispatchStart).Hours()
	if err := uc.repo.CompleteExecution(ctx, tenantID, in.TaskID, domain.StatusReview, actualHours); err != nil {
		return ExecuteResult{}, apperrors.New(apperrors.KindInternal, "TASK_EXECUTE_COMPLETION_WRITE_FAILED", "failed to persist execution completion", err)
	}
	return ExecuteResult{ExecutionRef: result, Async: false}, nil
}
```

Add `ExecuteResult{ ExecutionRef string; Async bool }` as a new exported
type in `execute_task.go`, replacing the bare `string` return `Execute`
currently has — update `server.go`'s `Execute` handler and
`TaskServiceExecuteResponse` (add an `async` bool field to that proto
message, a small additive change alongside this task) accordingly.

Add `CompleteExecution` to `TaskRepository` in `ports.go`:

```go
	// CompleteExecution is the simple path's (and, via TASK-TG-04-05's
	// ReportTaskExecutionResult, the complex path's) terminal write: sets
	// status, actual_hours, and clears agent_session_id in one statement.
	CompleteExecution(ctx context.Context, tenantID, id, status string, actualHours float64) error
```

Implement it in `repository.go`:

```go
func (r *Repository) CompleteExecution(ctx context.Context, tenantID, id, status string, actualHours float64) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE task.tasks SET status = $3, actual_hours = $4, agent_session_id = NULL, updated_at = now()
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, id, status, actualHours)
	if err != nil {
		return fmt.Errorf("postgres: complete task execution: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("postgres: task %s not found", id)
	}
	return nil
}
```

Add a small `Clock` port to `ports.go` if `TASK-TG-03-06` hasn't already
added one (`type Clock interface { Now() time.Time }`), and a
`SystemClock` implementation wired in `main.go` (`type SystemClock struct{};
func (SystemClock) Now() time.Time { return time.Now() }`).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/task-service/...
go test ./services/task-service/internal/usecase/... -run TestExecuteTask -v
```

Expected: successful simple execution writes `StatusReview` + a non-zero
`actual_hours` inline, in the same `Execute` call, with no second RPC;
complex-path success returns `Async: true` and leaves status at
`InProgress` (no inline completion); a task with an existing `WorktreeID`
never calls `EnsureWorktree`'s create branch (via `WorktreeProvisioner`
fake asserting `CreateWorktree` not invoked).
