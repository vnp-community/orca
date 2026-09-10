package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// GetSubtree returns id and every descendant of id in one repository round
// trip — see TaskRepository.GetSubtree's doc comment.
type GetSubtree struct {
	tasks TaskRepository
}

func NewGetSubtree(tasks TaskRepository) *GetSubtree {
	return &GetSubtree{tasks: tasks}
}

func (uc *GetSubtree) Execute(ctx context.Context, id string) ([]domain.Task, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}
	tasks, err := uc.tasks.GetSubtree(ctx, tenantID, id)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "TASK_GET_SUBTREE_FAILED", "failed to load task subtree", err)
	}
	return tasks, nil
}
