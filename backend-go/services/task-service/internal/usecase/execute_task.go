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
// SimpleExecutor (infra-fleet-service). Both executors are STUBS in this
// scaffold (internal/adapter/grpcclient) — only the branch decision itself
// is real.
type ExecuteTask struct {
	edges   EdgeRepository
	simple  SimpleExecutor
	complex ComplexExecutor
}

func NewExecuteTask(edges EdgeRepository, simple SimpleExecutor, complex ComplexExecutor) *ExecuteTask {
	return &ExecuteTask{edges: edges, simple: simple, complex: complex}
}

func (uc *ExecuteTask) Execute(ctx context.Context, in ExecuteTaskInput) (string, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return "", apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}
	if in.TaskID == "" {
		return "", apperrors.New(apperrors.KindInvalidArgument, "TASK_EXECUTE_INVALID", "task_id is required", nil)
	}

	complex, err := uc.isComplex(ctx, tenantID, in.TaskID)
	if err != nil {
		return "", apperrors.New(apperrors.KindInternal, "TASK_EXECUTE_EDGE_LOOKUP_FAILED", "failed to determine task complexity", err)
	}

	var ref string
	if complex {
		ref, err = uc.complex.Execute(ctx, tenantID, in.TaskID, in.RequestID)
	} else {
		ref, err = uc.simple.Execute(ctx, tenantID, in.TaskID, in.RequestID)
	}
	if err != nil {
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
