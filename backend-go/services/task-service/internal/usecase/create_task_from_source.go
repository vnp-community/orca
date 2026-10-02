package usecase

import (
	"context"
	"errors"
	"log/slog"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// CreateTaskFromSourceInput is CreateTaskInput plus the external issue the
// task is started from.
type CreateTaskFromSourceInput struct {
	CreateTaskInput
	Provider string
	Ref      string
	URL      string
}

// CreateTaskFromSourceResult.Created is false when an earlier "start work" on
// the same issue already produced the task.
type CreateTaskFromSourceResult struct {
	Task    domain.Task
	Created bool
}

// CreateTaskFromSource makes "start work on this issue" idempotent: one task
// per (tenant, project, provider, ref). Without it, a double click or a second
// teammate starting the same Jira issue would fork a duplicate task and a
// duplicate worktree.
type CreateTaskFromSource struct {
	repo    TaskRepository
	sources TaskSourceRepository
	create  *CreateTask
}

func NewCreateTaskFromSource(repo TaskRepository, sources TaskSourceRepository, create *CreateTask) *CreateTaskFromSource {
	return &CreateTaskFromSource{repo: repo, sources: sources, create: create}
}

func (uc *CreateTaskFromSource) Execute(ctx context.Context, in CreateTaskFromSourceInput) (CreateTaskFromSourceResult, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return CreateTaskFromSourceResult{}, apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}
	src, err := domain.NewTaskSource(tenantID, in.ProjectID, domain.SourceProvider(in.Provider), in.Ref, in.URL)
	if err != nil {
		return CreateTaskFromSourceResult{}, apperrors.New(apperrors.KindInvalidArgument, "TASK_SOURCE_INVALID", err.Error(), err)
	}

	if existing, ok, err := uc.existing(ctx, tenantID, src); err != nil {
		return CreateTaskFromSourceResult{}, err
	} else if ok {
		return CreateTaskFromSourceResult{Task: existing}, nil
	}

	created, err := uc.create.Execute(ctx, in.CreateTaskInput)
	if err != nil {
		return CreateTaskFromSourceResult{}, err
	}
	src.TaskID = created.ID
	if err := uc.sources.LinkSource(ctx, src); err != nil {
		// Roll back the orphan either way; a leftover sourceless task would
		// show up as a duplicate on the board.
		if delErr := uc.repo.Delete(ctx, tenantID, created.ID); delErr != nil {
			slog.ErrorContext(ctx, "create_task_from_source: failed to remove orphan task", slog.String("task_id", created.ID), slog.Any("error", delErr))
		}
		if errors.Is(err, domain.ErrSourceAlreadyLinked) {
			// Lost the race to a concurrent start on the same issue: return the winner.
			winner, ok, ferr := uc.existing(ctx, tenantID, src)
			if ferr != nil {
				return CreateTaskFromSourceResult{}, ferr
			}
			if ok {
				return CreateTaskFromSourceResult{Task: winner}, nil
			}
		}
		return CreateTaskFromSourceResult{}, apperrors.New(apperrors.KindInternal, "TASK_SOURCE_LINK_FAILED", "failed to link task to source", err)
	}
	return CreateTaskFromSourceResult{Task: created, Created: true}, nil
}

func (uc *CreateTaskFromSource) existing(ctx context.Context, tenantID string, src domain.TaskSource) (domain.Task, bool, error) {
	taskID, ok, err := uc.sources.FindTaskIDBySource(ctx, tenantID, src.ProjectID, src.Provider, src.Ref)
	if err != nil {
		return domain.Task{}, false, apperrors.New(apperrors.KindInternal, "TASK_SOURCE_LOOKUP_FAILED", "failed to look up task by source", err)
	}
	if !ok {
		return domain.Task{}, false, nil
	}
	task, err := uc.repo.Get(ctx, tenantID, taskID)
	if err != nil {
		return domain.Task{}, false, apperrors.New(apperrors.KindInternal, "TASK_SOURCE_TASK_LOAD_FAILED", "failed to load task linked to source", err)
	}
	return task, true, nil
}
