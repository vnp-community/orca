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
	ProjectID  string
	PageToken  string
	PageSize   int32
	TaskTypes  []string
	RequestIDs []string
	ParentID   string
}

// maxListRequestIDs bounds the IN list sent to the database.
const maxListRequestIDs = 100

// defaultListTaskTypes keeps plan/phase out of listings from clients that predate them.
var defaultListTaskTypes = []string{domain.TypeTask, domain.TypeBug, domain.TypeFeature, domain.TypeEpic}

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
	if len(in.RequestIDs) > maxListRequestIDs {
		return ListTasksResult{}, apperrors.New(apperrors.KindInvalidArgument, "TASK_LIST_TOO_MANY_REQUEST_IDS", "too many request_ids (max 100)", nil)
	}
	var types []string
	for _, raw := range in.TaskTypes {
		if raw == "" {
			continue
		}
		tt, err := domain.ParseTaskType(raw)
		if err != nil {
			return ListTasksResult{}, apperrors.New(apperrors.KindInvalidArgument, "TASK_INVALID_TYPE", "invalid task type: "+raw, err)
		}
		types = append(types, tt)
	}
	if len(types) == 0 {
		types = append([]string(nil), defaultListTaskTypes...)
	}
	tasks, nextToken, err := uc.repo.List(ctx, tenantID, ListFilter{
		ProjectID: in.ProjectID, TaskTypes: types, RequestIDs: in.RequestIDs, ParentID: in.ParentID,
		PageToken: in.PageToken, PageSize: in.PageSize,
	})
	if err != nil {
		return ListTasksResult{}, apperrors.New(apperrors.KindInternal, "TASK_LIST_FAILED", "failed to list tasks", err)
	}
	return ListTasksResult{Tasks: tasks, NextPageToken: nextToken}, nil
}
