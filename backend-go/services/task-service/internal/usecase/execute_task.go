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

// ExecuteResult replaces the bare execution-ref string Execute used to
// return — Async distinguishes the complex path (orchestration-service,
// completion arrives later via ReportTaskExecutionResult, TASK-TG-04-05)
// from the simple path (SimpleExecutor.Execute blocks until the CLI
// process exits, so completion is written INLINE, same call — TASK-TG-04-03).
type ExecuteResult struct {
	ExecutionRef string
	Async        bool
}

// ExecuteTask is task-service's execution-dispatch usecase (§3.1). The
// branching logic — simple vs. complex — is this usecase's real,
// non-stubbed value: a task with any parent_child (subtask) or depends_on
// edge FROM it is "complex" and hands off to ComplexExecutor
// (orchestration-service); otherwise it's "simple" and relays directly to
// SimpleExecutor (infra-fleet-service).
//
// TASK-TG-04-01 fixed two real bugs found while grounding SOL-TG-04
// against this code: the permission check and complexity determination now
// run BEFORE the in_progress write, and a dispatch failure reverts to the
// task's PREVIOUS status (a compensating write) rather than leaving it
// permanently stuck in_progress.
//
// TASK-TG-04-03 adds two more real pieces: (1) worktree reuse-or-create via
// WorktreeProvisioner before dispatch, persisting Task.WorktreeID when it
// changes; (2) inline completion for the simple path — SimpleExecutor.Execute
// already blocks until the Dev Server Agent's CLI process exits (see
// SimpleExecutor's own doc comment), so ExecuteTask already knows the
// simple path finished, successfully or not, before its own Execute call
// returns. That's a completion callback available and unused until now:
// on simple-path success this writes StatusReview + actual_hours (measured
// via Clock) in the SAME call, no second RPC. The complex path has no such
// synchronous signal (orchestration-service's dispatch is async) — it stays
// at StatusInProgress until ReportTaskExecutionResult (TASK-TG-04-05).
type ExecuteTask struct {
	repo              TaskRepository
	edges             EdgeRepository
	simple            SimpleExecutor
	complex           ComplexExecutor
	resolvePermission *ResolvePermission
	worktrees         WorktreeProvisioner
	resolver          ProjectExecutionResolver
	clock             Clock
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

	// Pre-check 1: permission — every mutating RPC calls ResolvePermission
	// internally first per task-service.md §3; ExecuteTask never has,
	// despite dispatching real work. Closed here, BEFORE any status write.
	userID, _ := tenant.UserID(ctx)
	if _, err := uc.resolvePermission.Execute(ctx, ResolvePermissionInput{TaskID: in.TaskID, UserID: userID, Action: "execute"}); err != nil {
		return ExecuteResult{}, err // PermissionDenied, no status write happened yet
	}

	task, err := uc.repo.Get(ctx, tenantID, in.TaskID)
	if err != nil {
		return ExecuteResult{}, apperrors.New(apperrors.KindNotFound, "TASK_NOT_FOUND", "task not found", err)
	}
	previousStatus := task.Status

	complex, err := uc.isComplex(ctx, tenantID, in.TaskID) // unchanged — computed BEFORE any status write now, not after
	if err != nil {
		return ExecuteResult{}, apperrors.New(apperrors.KindInternal, "TASK_EXECUTE_EDGE_LOOKUP_FAILED", "failed to determine task complexity", err)
	}

	// Pre-check 2: dev-server-online — resolved once here, BEFORE the
	// in_progress write, so a disconnected project fails before any status
	// mutation at all.
	_, resolvedPath, _, connected, err := uc.resolver.ResolveConnection(ctx, tenantID, task.ProjectID)
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
	_ = worktreePath // resolved for parity with SOL-TG-04's design; SimpleExecutor/ComplexExecutor resolve their own worktree path today (TASK-TG-04-06 threads this through as a context preamble)
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
			// The fix: revert the in_progress write instead of leaving the task
			// stuck — a dispatch failure must never leave permanently-false
			// "running" state, since there is no other RPC to clear it.
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

// isComplex implements task-service.md §3.1's branch: "a complex task has
// subtasks and/or dependency edges."
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
