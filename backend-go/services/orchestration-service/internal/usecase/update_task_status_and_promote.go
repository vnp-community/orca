package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"

	"github.com/stablyai/orca-go/services/orchestration-service/internal/domain"
)

// taskStatusChangedPayload is orca.orchestration.task.statuschanged's JSON
// payload shape (BE-SOL-003/TASK-FT-003-01). OriginTaskID is carried
// directly so api-gateway's task.activity channel (TASK-FT-003-04) can
// filter by it without a cross-service lookup.
type taskStatusChangedPayload struct {
	OrchestrationTaskID string `json:"orchestration_task_id"`
	CoordinatorRunID    string `json:"coordinator_run_id"`
	OriginTaskID        string `json:"origin_task_id"`
	NewStatus           string `json:"new_status"`
}

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
	reporter   TaskServiceReporter
	runs       CoordinatorRunRepository // only for MarkReported after a successful report
}

func NewUpdateTaskStatusAndPromote(repo OrchestrationTaskRepository, serializer HandleSerializer, reporter TaskServiceReporter, runs CoordinatorRunRepository) *UpdateTaskStatusAndPromote {
	return &UpdateTaskStatusAndPromote{repo: repo, serializer: serializer, reporter: reporter, runs: runs}
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

	// The outbox event's payload needs the task's CoordinatorRunID/
	// OriginTaskID (BE-SOL-003, TASK-FT-003-04's task.activity filter) —
	// read via Get before the status-changing call below, since
	// UpdateStatusAndPromote's transaction only returns the row AFTER its
	// write and this usecase must build the event BEFORE calling it
	// (TASK-FT-003-01's usage-service-pattern correction: the usecase
	// builds domain.OutboxEvent, the repository enqueues it in the same
	// transaction as the domain write). Safe to read outside the
	// serializer/transaction because coordinator_run_id/origin_task_id are
	// immutable after a task is created — only status/completed_at mutate
	// — so a stale read of them poses no correctness risk; a failure here
	// (including "task not found") just degrades to skipping the event,
	// since UpdateStatusAndPromote below still returns the real
	// ErrTaskNotFound for the RPC's own error handling.
	var event domain.OutboxEvent
	if existing, gerr := uc.repo.Get(ctx, tenantID, in.OrchestrationTaskID); gerr == nil {
		payload, merr := json.Marshal(taskStatusChangedPayload{
			OrchestrationTaskID: in.OrchestrationTaskID,
			CoordinatorRunID:    existing.CoordinatorRunID,
			OriginTaskID:        existing.OriginTaskID,
			NewStatus:           string(newStatus),
		})
		if merr == nil {
			event = domain.OutboxEvent{
				ID: uuid.NewString(), Subject: "orca.orchestration.task.statuschanged",
				OccurredAt: time.Now().UTC(), PayloadJSON: payload,
			}
		}
		// A marshal failure degrades to "persist the status change, skip
		// the event" — same best-effort posture workflow-service's
		// runToCompletion already uses for its own terminal-event marshal
		// failure.
	}

	var result UpdateStatusAndPromoteResult
	err = uc.serializer.Do(ctx, in.OrchestrationTaskID, func() error {
		r, err := uc.repo.UpdateStatusAndPromote(ctx, tenantID, in.OrchestrationTaskID, newStatus, event)
		if err != nil {
			return err
		}
		result = r
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrTaskNotFound) {
			return UpdateTaskStatusAndPromoteOutput{}, apperrors.New(apperrors.KindNotFound, "ORCH_TASK_NOT_FOUND", "orchestration task not found", err)
		}
		return UpdateTaskStatusAndPromoteOutput{}, apperrors.New(apperrors.KindInternal, "ORCH_UPDATE_TASK_STATUS_FAILED", "failed to update task status and promote", err)
	}

	// Reporting happens AFTER the transaction has committed — per
	// ReportTaskExecutionResult's own idempotence note (SOL-TG-04:
	// "ignored, not an error... at-least-once consumer idempotence"), a
	// reporter-call failure here is logged and left for the tick loop's
	// retry pass (TASK-TASKV1-005-10, ListUnreportedTerminal) rather than
	// failing this whole RPC — the task-status write itself already
	// committed successfully and must not be rolled back by a downstream
	// notification failure.
	if result.RunFinalized != nil {
		f := result.RunFinalized
		if err := uc.reporter.ReportResult(ctx, f.OriginTaskID, f.CoordinatorRunID, f.Success, ""); err != nil {
			slog.Error("report coordinator run result failed, will retry via tick loop",
				slog.String("coordinator_run_id", f.CoordinatorRunID), slog.Any("error", err))
		} else if err := uc.runs.MarkReported(ctx, tenantID, f.CoordinatorRunID); err != nil {
			slog.Error("mark coordinator run reported failed", slog.String("coordinator_run_id", f.CoordinatorRunID), slog.Any("error", err))
		}
	}

	return UpdateTaskStatusAndPromoteOutput{Task: result.Task, PromotedTaskIDs: result.PromotedIDs}, nil
}
