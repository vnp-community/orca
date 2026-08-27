package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// ListTasksInput mirrors the ListTasks RPC request — see CreateTaskInput's
// doc comment for why TenantID isn't a field here: it's pulled from
// context (common/tenant), matching every other usecase in this package.
type ListTasksInput struct {
	ProjectID string
	PageToken string
	PageSize  int32
}

type ListTasksResult struct {
	Tasks         []domain.Task
	NextPageToken string
}

type ListTasks struct {
	repo TaskRepository
}

func NewListTasks(repo TaskRepository) *ListTasks {
	return &ListTasks{repo: repo}
}

func (uc *ListTasks) Execute(ctx context.Context, in ListTasksInput) (ListTasksResult, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return ListTasksResult{}, apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}
	tasks, nextToken, err := uc.repo.List(ctx, tenantID, in.ProjectID, in.PageToken, in.PageSize)
	if err != nil {
		return ListTasksResult{}, apperrors.New(apperrors.KindInternal, "TASK_LIST_FAILED", "failed to list tasks", err)
	}
	return ListTasksResult{Tasks: tasks, NextPageToken: nextToken}, nil
}
