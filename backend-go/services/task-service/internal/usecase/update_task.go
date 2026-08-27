package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// UpdateTaskInput mirrors the UpdateTask RPC request's wrapper-typed
// optional fields — a status-only edit shouldn't require resending Title.
// See CreateTaskInput's doc comment for why TenantID isn't a field here.
type UpdateTaskInput struct {
	ID     string
	Title  *string
	Status *string
}

// UpdateTask is task-service's one client-facing status-edit RPC. It
// deliberately does NOT become the general mechanism that clears
// StatusInProgress back out (the one-way-transition gap execute_task.go's
// doc comment names) — domain.Task.SetStatus rejects any transition into
// in_progress here, so a buggy or malicious client can't mark a
// still-running task done early or fake a dispatch it never made. See
// TASK-223's Context note.
type UpdateTask struct {
	repo  TaskRepository
	edges EdgeRepository
}

func NewUpdateTask(repo TaskRepository, edges EdgeRepository) *UpdateTask {
	return &UpdateTask{repo: repo, edges: edges}
}

func (uc *UpdateTask) Execute(ctx context.Context, in UpdateTaskInput) (domain.Task, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.Task{}, apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}
	if in.ID == "" {
		return domain.Task{}, apperrors.New(apperrors.KindInvalidArgument, "TASK_MISSING_ID", "id is required", nil)
	}

	current, err := uc.repo.Get(ctx, tenantID, in.ID)
	if err != nil {
		return domain.Task{}, apperrors.New(apperrors.KindNotFound, "TASK_NOT_FOUND", "task not found", err)
	}
	if in.Title != nil {
		current.Title = *in.Title
	}
	if in.Status != nil {
		updated, err := current.SetStatus(*in.Status)
		if err != nil {
			return domain.Task{}, apperrors.New(apperrors.KindInvalidArgument, "TASK_INVALID_STATUS_TRANSITION", err.Error(), err)
		}
		current = updated
	}
	if err := uc.repo.Update(ctx, tenantID, current); err != nil {
		return domain.Task{}, apperrors.New(apperrors.KindInternal, "TASK_UPDATE_FAILED", "failed to persist update", err)
	}

	// Un-block step: a task transitioning to Done may unblock direct
	// dependents whose every depends_on edge is now satisfied.
	if in.Status != nil && *in.Status == domain.StatusDone {
		dependents, err := uc.edges.ListTo(ctx, tenantID, current.ID, domain.EdgeKindDependsOn)
		if err != nil {
			return domain.Task{}, apperrors.New(apperrors.KindInternal, "TASK_UNBLOCK_LOOKUP_FAILED", "failed to list dependents for unblock check", err)
		}
		for _, dep := range dependents {
			dependent, err := uc.repo.Get(ctx, tenantID, dep.FromTaskID)
			if err != nil || dependent.Status != domain.StatusBlocked {
				continue
			}
			blockers, err := uc.edges.ListFrom(ctx, tenantID, dependent.ID, domain.EdgeKindDependsOn)
			if err != nil {
				continue
			}
			allDone := true
			for _, b := range blockers {
				blocker, err := uc.repo.Get(ctx, tenantID, b.ToTaskID)
				if err != nil || (blocker.Status != domain.StatusDone && blocker.Status != domain.StatusCancelled) {
					allDone = false
					break
				}
			}
			if allDone {
				_ = uc.repo.UpdateStatus(ctx, tenantID, dependent.ID, domain.StatusOpen)
			}
		}
	}
	return current, nil
}
