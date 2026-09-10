package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// RecalculateProgress reduces a subtree bottom-up (deepest-first) through
// domain.CalculateProgress, then persists every changed value in ONE
// batched write — task-service.md §8's "one WITH RECURSIVE aggregate query
// rather than N+1 fetches" NFR.
type RecalculateProgress struct {
	tasks TaskRepository
}

func NewRecalculateProgress(tasks TaskRepository) *RecalculateProgress {
	return &RecalculateProgress{tasks: tasks}
}

func (uc *RecalculateProgress) Execute(ctx context.Context, rootID string) (int, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return 0, apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}

	subtree, err := uc.tasks.GetSubtreeWithChildPercents(ctx, tenantID, rootID)
	if err != nil {
		return 0, apperrors.New(apperrors.KindNotFound, "TASK_NOT_FOUND", "task not found while recalculating progress", err)
	}

	updates := make(map[string]int, len(subtree))
	childPercentsByParent := map[string][]int{}
	rootPercent := 0
	for _, node := range subtree { // already ordered deepest-first by the repository query
		children := childPercentsByParent[node.Task.ID]
		p := domain.CalculateProgress(node.Task, children)
		updates[node.Task.ID] = p
		if node.Task.ParentID != "" {
			childPercentsByParent[node.Task.ParentID] = append(childPercentsByParent[node.Task.ParentID], p)
		}
		if node.Task.ID == rootID {
			rootPercent = p
		}
	}

	if err := uc.tasks.BatchUpdateProgress(ctx, tenantID, updates); err != nil {
		return 0, apperrors.New(apperrors.KindInternal, "TASK_PROGRESS_UPDATE_FAILED", "failed to persist recalculated progress", err)
	}
	return rootPercent, nil
}
