package usecase

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// UpdateTaskInput mirrors the UpdateTask RPC request's wrapper-typed
// optional fields — a status-only edit shouldn't require resending Title.
// See CreateTaskInput's doc comment for why TenantID isn't a field here.
// PRURL/WorktreeID added SOL-PW-04 for the PR-creation write-back saga and
// task<->worktree linkage.
type UpdateTaskInput struct {
	ID         string
	Title      *string
	Status     *string
	PRURL      *string
	WorktreeID *string
}

// UpdateTask is task-service's one client-facing status-edit RPC. It
// deliberately does NOT become the general mechanism that clears
// StatusInProgress back out (the one-way-transition gap execute_task.go's
// doc comment names) — domain.Task.SetStatus rejects any transition into
// in_progress here, so a buggy or malicious client can't mark a
// still-running task done early or fake a dispatch it never made. See
// TASK-223's Context note.
//
// SOL-PW-04: a genuine status transition also builds an OutboxEvent and
// passes it to Update, which persists it in the SAME transaction as the
// status write — this is task-service's half of the previously-missing
// cross-service event bus (git-gateway-service.md's outbox precedent,
// TASK-PW-04-02). Two subjects can be enqueued from one call:
// orca.task.task.statuschanged always (every transition — feeds
// api-gateway's workspace event bridge), and additionally
// orca.task.task.completed when the new status is domain.StatusDone (feeds
// notification-service's ALREADY-WIRED consumer/rule — verified present in
// consumer.go/notification_event.go, not assumed — so that subject name is
// preserved exactly, not renamed to statuschanged's shape).
type UpdateTask struct {
	repo TaskRepository
}

func NewUpdateTask(repo TaskRepository) *UpdateTask {
	return &UpdateTask{repo: repo}
}

func (uc *UpdateTask) Execute(ctx context.Context, in UpdateTaskInput) (domain.Task, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.Task{}, apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}
	if in.ID == "" {
		return domain.Task{}, apperrors.New(apperrors.KindInvalidArgument, "TASK_MISSING_ID", "id is required", nil)
	}

	current, err := uc.repo.Get(ctx, tenantID, in.ID)
	if err != nil {
		return domain.Task{}, apperrors.New(apperrors.KindNotFound, "TASK_NOT_FOUND", "task not found", err)
	}
	previousStatus := current.Status
	if in.Title != nil {
		current.Title = *in.Title
	}
	if in.PRURL != nil {
		current.PRURL = *in.PRURL
	}
	if in.WorktreeID != nil {
		current.WorktreeID = *in.WorktreeID
	}
	if in.Status != nil {
		updated, err := current.SetStatus(*in.Status)
		if err != nil {
			return domain.Task{}, apperrors.New(apperrors.KindInvalidArgument, "TASK_INVALID_STATUS_TRANSITION", err.Error(), err)
		}
		current = updated
	}

	var events []domain.OutboxEvent
	if in.Status != nil && *in.Status != previousStatus {
		payload, err := json.Marshal(taskStatusChangedPayload{
			TaskID: current.ID, ProjectID: current.ProjectID, WorktreeID: current.WorktreeID,
			PreviousStatus: previousStatus, NewStatus: current.Status,
		})
		if err != nil {
			return domain.Task{}, apperrors.New(apperrors.KindInternal, "TASK_MARSHAL_EVENT_FAILED", "failed to marshal status-changed event payload", err)
		}
		events = append(events, domain.OutboxEvent{ID: uuid.NewString(), Subject: "orca.task.task.statuschanged", OccurredAt: time.Now().UTC(), PayloadJSON: payload})
		if current.Status == domain.StatusDone {
			// Feeds notification-service's ALREADY-WIRED
			// "orca.task.task.completed" consumer/rule — verified present
			// in consumer.go/notification_event.go before assuming this
			// subject name; do not rename it to statuschanged's shape.
			events = append(events, domain.OutboxEvent{ID: uuid.NewString(), Subject: "orca.task.task.completed", OccurredAt: time.Now().UTC(), PayloadJSON: payload})
		}
	}

	if err := uc.repo.Update(ctx, tenantID, current, events); err != nil {
		return domain.Task{}, apperrors.New(apperrors.KindInternal, "TASK_UPDATE_FAILED", "failed to persist update", err)
	}
	return current, nil
}

type taskStatusChangedPayload struct {
	TaskID         string `json:"task_id"`
	ProjectID      string `json:"project_id"`
	WorktreeID     string `json:"worktree_id"`
	PreviousStatus string `json:"previous_status"`
	NewStatus      string `json:"new_status"`
}
