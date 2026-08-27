package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

type GetTask struct {
	repo TaskRepository
}

func NewGetTask(repo TaskRepository) *GetTask {
	return &GetTask{repo: repo}
}

func (uc *GetTask) Execute(ctx context.Context, id string) (domain.Task, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.Task{}, apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}

	task, err := uc.repo.Get(ctx, tenantID, id)
	if err != nil {
		return domain.Task{}, apperrors.New(apperrors.KindNotFound, "TASK_NOT_FOUND", "task not found", err)
	}
	return task, nil
}
