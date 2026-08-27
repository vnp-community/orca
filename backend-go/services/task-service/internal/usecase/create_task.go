package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// CreateTaskInput mirrors the CreateTask RPC request 1:1 by design — see
// architecture/03's note that usecase granularity mirrors today's RPC
// methods so the TS->Go mapping stays traceable. TenantID is NOT part of
// this struct: it's pulled from context (common/tenant), never trusted from
// the request body, per architecture/05-data-architecture.md. ID is
// optional: the CreateTaskRequest proto message has no id field (the
// service owns ID assignment), but tests may supply one directly for
// determinism.
type CreateTaskInput struct {
	ID        string
	Title     string
	ParentID  string
	ProjectID string
}

type CreateTask struct {
	repo TaskRepository
}

func NewCreateTask(repo TaskRepository) *CreateTask {
	return &CreateTask{repo: repo}
}

func (uc *CreateTask) Execute(ctx context.Context, in CreateTaskInput) (domain.Task, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.Task{}, apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}

	id := in.ID
	if id == "" {
		id = uuid.NewString()
	}
	task, err := domain.NewTask(id, tenantID, in.Title, domain.StatusOpen, in.ParentID, in.ProjectID)
	if err != nil {
		return domain.Task{}, apperrors.New(apperrors.KindInvalidArgument, "TASK_INVALID", err.Error(), err)
	}

	if task.ParentID != "" {
		if _, err := uc.repo.Get(ctx, tenantID, task.ParentID); err != nil {
			return domain.Task{}, apperrors.New(apperrors.KindFailedPrecondition, "TASK_PARENT_NOT_FOUND", "parent task does not exist", err)
		}
	}

	created, err := uc.repo.Create(ctx, task)
	if err != nil {
		return domain.Task{}, apperrors.New(apperrors.KindInternal, "TASK_CREATE_FAILED", "failed to persist task", err)
	}
	return created, nil
}
