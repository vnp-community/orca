package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

// DeleteTaskInput mirrors the DeleteTask RPC request. See CreateTaskInput's
// doc comment for why TenantID isn't a field here.
type DeleteTaskInput struct {
	ID string
}

type DeleteTask struct {
	repo TaskRepository
}

func NewDeleteTask(repo TaskRepository) *DeleteTask {
	return &DeleteTask{repo: repo}
}

func (uc *DeleteTask) Execute(ctx context.Context, in DeleteTaskInput) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}
	if in.ID == "" {
		return apperrors.New(apperrors.KindInvalidArgument, "TASK_MISSING_ID", "id is required", nil)
	}
	if err := uc.repo.Delete(ctx, tenantID, in.ID); err != nil {
		return apperrors.New(apperrors.KindInternal, "TASK_DELETE_FAILED", "failed to delete task", err)
	}
	return nil
}
