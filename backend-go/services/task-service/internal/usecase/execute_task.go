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

// ExecuteTask is task-service's execution-dispatch usecase (§3.1). The
// branching logic — simple vs. complex — is this usecase's real,
// non-stubbed value: a task with any parent_child (subtask) or depends_on
// edge FROM it is "complex" and hands off to ComplexExecutor
// (orchestration-service); otherwise it's "simple" and relays directly to
// SimpleExecutor (infra-fleet-service).
//
// Execute marks the task StatusInProgress (via TaskRepository) before
// dispatching — a real, persisted state transition added for Epic C
// (backend-go/docs/execution-plan.md) so usecase.HasActiveExecutions has
// something true to query. TASK-TG-04-01 fixed two real bugs found while
// grounding SOL-TG-04 against this code: (1) the in_progress write used to
// happen BEFORE permission checking and complexity determination, so a
// dispatch that never should have started still flipped the status first;
// (2) a dispatch failure (e.g. the project's dev server offline) never
// reverted that write, permanently stranding the task in_progress with no
// RPC to clear it. Both are fixed here: the permission check and
// complexity determination now run BEFORE the in_progress write, and a
// dispatch failure reverts to the task's PREVIOUS status (a compensating
// write) rather than leaving it stuck.
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
