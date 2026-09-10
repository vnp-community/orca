package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// FindTaskByNumberInput resolves a project-scoped "#TG-N" reference to a
// task id — the commit-message-closes-task saga's lookup. Project-scoped,
// not tenant-wide: two different projects can each have their own #TG-42,
// matching how GitHub/Jira issue numbers are repo/project-scoped, not
// org-wide.
type FindTaskByNumberInput struct {
	ProjectID  string
	TaskNumber int64
}

type FindTaskByNumber struct {
	repo TaskRepository
}

func NewFindTaskByNumber(repo TaskRepository) *FindTaskByNumber {
	return &FindTaskByNumber{repo: repo}
}

func (uc *FindTaskByNumber) Execute(ctx context.Context, in FindTaskByNumberInput) (domain.Task, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.Task{}, apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}
	if in.ProjectID == "" {
		return domain.Task{}, apperrors.New(apperrors.KindInvalidArgument, "TASK_MISSING_PROJECT_ID", "project_id is required", nil)
	}
	task, err := uc.repo.FindByNumber(ctx, tenantID, in.ProjectID, in.TaskNumber)
	if err != nil {
		return domain.Task{}, apperrors.New(apperrors.KindNotFound, "TASK_NOT_FOUND_BY_NUMBER", "no task with this number in this project", err)
	}
	return task, nil
}
