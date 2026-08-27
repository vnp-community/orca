package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"

	"github.com/stablyai/orca-go/services/orchestration-service/internal/domain"
)

// UpdateTaskStatusAndPromoteInput mirrors the UpdateTaskStatusAndPromoteRequest
// RPC message. Collapses TS's updateTaskStatus -> promoteReadyTasks chain
// into one call so the atomicity requirement is structural (enforced by the
// repository's single transaction), not a convention callers must remember.
type UpdateTaskStatusAndPromoteInput struct {
	OrchestrationTaskID string
	NewStatus           string
}

// UpdateTaskStatusAndPromoteOutput carries the updated task and the ids of
// any sibling tasks promoted to ready as a result.
type UpdateTaskStatusAndPromoteOutput struct {
	Task            domain.OrchestrationTask
	PromotedTaskIDs []string
}

// UpdateTaskStatusAndPromote is keyed by OrchestrationTaskID.
//
// orchestration-service.md §6's diagram keys this chain by assignee_handle
// (the domain-event handler that triggers a promote knows which worker's
// message caused it). The generated UpdateTaskStatusAndPromoteRequest proto
// message, however, carries only orchestration_task_id and new_status — no
// handle. Keying by OrchestrationTaskID instead is the closest available
// substitute: it still closes the real race this exists to prevent (two
// concurrent/retried UpdateTaskStatusAndPromote calls for the SAME task
// interleaving their promotion scans), even though it's narrower than a
// handle-wide serialization scope.
type UpdateTaskStatusAndPromote struct {
	repo       OrchestrationTaskRepository
	serializer HandleSerializer
}

func NewUpdateTaskStatusAndPromote(repo OrchestrationTaskRepository, serializer HandleSerializer) *UpdateTaskStatusAndPromote {
	return &UpdateTaskStatusAndPromote{repo: repo, serializer: serializer}
}

func (uc *UpdateTaskStatusAndPromote) Execute(ctx context.Context, in UpdateTaskStatusAndPromoteInput) (UpdateTaskStatusAndPromoteOutput, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return UpdateTaskStatusAndPromoteOutput{}, apperrors.New(apperrors.KindUnauthenticated, "ORCH_NO_TENANT", "no tenant in request context", err)
	}
	if in.OrchestrationTaskID == "" {
		return UpdateTaskStatusAndPromoteOutput{}, apperrors.New(apperrors.KindInvalidArgument, "ORCH_EMPTY_TASK_ID", "orchestration_task_id is required", nil)
	}
	newStatus := domain.TaskStatus(in.NewStatus)
	if !newStatus.Valid() {
		return UpdateTaskStatusAndPromoteOutput{}, apperrors.New(apperrors.KindInvalidArgument, "ORCH_INVALID_STATUS", "new_status is not a valid task status", domain.ErrInvalidTaskStatus)
	}

	var out UpdateTaskStatusAndPromoteOutput
	err = uc.serializer.Do(ctx, in.OrchestrationTaskID, func() error {
		task, promoted, err := uc.repo.UpdateStatusAndPromote(ctx, tenantID, in.OrchestrationTaskID, newStatus)
		if err != nil {
			return err
		}
		out = UpdateTaskStatusAndPromoteOutput{Task: task, PromotedTaskIDs: promoted}
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrTaskNotFound) {
			return UpdateTaskStatusAndPromoteOutput{}, apperrors.New(apperrors.KindNotFound, "ORCH_TASK_NOT_FOUND", "orchestration task not found", err)
		}
		return UpdateTaskStatusAndPromoteOutput{}, apperrors.New(apperrors.KindInternal, "ORCH_UPDATE_TASK_STATUS_FAILED", "failed to update task status and promote", err)
	}
	return out, nil
}
