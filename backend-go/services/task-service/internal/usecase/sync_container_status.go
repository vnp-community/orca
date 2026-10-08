package usecase

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// maxContainerSyncAttempts bounds CAS retries when concurrent writers race on one container.
const maxContainerSyncAttempts = 3

// SyncContainerStatus recomputes plan/phase statuses from their children after a child changed.
// Writes go through TaskRepository.UpdateContainerStatus, never SetStatus, so the
// ErrCannotSetInProgress guard on client-driven updates stays intact.
type SyncContainerStatus struct {
	repo     TaskRepository
	progress *RecalculateProgress
}

// NewSyncContainerStatus's progress may be nil (progress recalculation is then skipped).
func NewSyncContainerStatus(repo TaskRepository, progress *RecalculateProgress) *SyncContainerStatus {
	return &SyncContainerStatus{repo: repo, progress: progress}
}

// Execute re-derives the container ancestors of changedTaskID, bottom-up. A task whose
// parent is not a plan/phase is a no-op.
func (uc *SyncContainerStatus) Execute(ctx context.Context, changedTaskID string) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}
	task, err := uc.repo.Get(ctx, tenantID, changedTaskID)
	if err != nil {
		return apperrors.New(apperrors.KindNotFound, "TASK_NOT_FOUND", "task not found", err)
	}
	if task.ParentID == "" {
		return nil
	}
	_, err = uc.syncChain(ctx, tenantID, task.ParentID)
	return err
}

// SyncOne re-derives containerID itself and then its container ancestors; changed reports
// whether containerID's own status moved.
func (uc *SyncContainerStatus) SyncOne(ctx context.Context, containerID string) (bool, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return false, apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}
	return uc.syncChain(ctx, tenantID, containerID)
}

func (uc *SyncContainerStatus) syncChain(ctx context.Context, tenantID, startID string) (bool, error) {
	firstChanged, first, top := false, true, ""
	for id := startID; id != ""; {
		c, err := uc.repo.Get(ctx, tenantID, id)
		if err != nil {
			return firstChanged, apperrors.New(apperrors.KindNotFound, "TASK_NOT_FOUND", "container not found", err)
		}
		// A cancelled container is never re-derived, and nothing above it depends on this change.
		if !domain.IsContainerType(c.Type) || c.Status == domain.StatusCancelled {
			break
		}
		changed, err := uc.deriveOne(ctx, tenantID, c)
		if err != nil {
			return firstChanged, err
		}
		if first {
			firstChanged, first = changed, false
		}
		top, id = c.ID, c.ParentID
	}
	// One recalculation at the topmost container covers every level below it.
	if top != "" && uc.progress != nil {
		if _, err := uc.progress.Execute(ctx, top); err != nil {
			slog.WarnContext(ctx, "task: container progress recalculation failed", slog.String("task_id", top), slog.Any("error", err))
		}
	}
	return firstChanged, nil
}

func (uc *SyncContainerStatus) deriveOne(ctx context.Context, tenantID string, c domain.Task) (bool, error) {
	for attempt := 0; attempt < maxContainerSyncAttempts; attempt++ {
		statuses, err := uc.repo.ListChildStatuses(ctx, tenantID, c.ID)
		if err != nil {
			return false, apperrors.New(apperrors.KindInternal, "TASK_CONTAINER_SYNC_FAILED", "failed to list child statuses", err)
		}
		derived, ok := domain.DeriveContainerStatus(statuses)
		if !ok || derived == c.Status {
			return false, nil
		}
		payload, err := json.Marshal(taskStatusChangedPayload{
			TaskID: c.ID, ProjectID: c.ProjectID, WorktreeID: c.WorktreeID,
			PreviousStatus: string(c.Status), NewStatus: string(derived),
			TaskType: c.Type, ParentID: c.ParentID, RequestID: c.RequestID, Cause: causeDerived,
		})
		if err != nil {
			return false, apperrors.New(apperrors.KindInternal, "TASK_MARSHAL_EVENT_FAILED", "failed to marshal status-changed event payload", err)
		}
		// Only statuschanged: container completion must not fire the work-task "completed" notification.
		event := domain.OutboxEvent{ID: uuid.NewString(), Subject: "orca.task.task.statuschanged", OccurredAt: time.Now().UTC(), PayloadJSON: payload}
		changed, err := uc.repo.UpdateContainerStatus(ctx, tenantID, c.ID, c.Status, derived, []domain.OutboxEvent{event})
		if err != nil {
			return false, apperrors.New(apperrors.KindInternal, "TASK_CONTAINER_SYNC_FAILED", "failed to update container status", err)
		}
		if changed {
			if c.Status == domain.StatusDone && derived != domain.StatusDone {
				slog.WarnContext(ctx, "task: completed container reopened by a new unfinished child",
					slog.String("task_id", c.ID), slog.String("new_status", string(derived)))
			}
			return true, nil
		}
		// Lost the CAS: reload and derive again from the winner's state.
		c, err = uc.repo.Get(ctx, tenantID, c.ID)
		if err != nil {
			return false, apperrors.New(apperrors.KindNotFound, "TASK_NOT_FOUND", "container not found", err)
		}
		if c.Status == domain.StatusCancelled {
			return false, nil
		}
	}
	slog.WarnContext(ctx, "task: container status sync gave up after repeated CAS losses", slog.String("task_id", c.ID))
	return false, nil
}

// syncContainerParent is the best-effort call-site helper: a failed sync is logged, never
// fails the command that triggered it (reconcile repairs the container later).
func syncContainerParent(ctx context.Context, sync *SyncContainerStatus, taskID string) {
	if sync == nil {
		return
	}
	if err := sync.Execute(ctx, taskID); err != nil {
		slog.WarnContext(ctx, "task: container status sync failed", slog.String("task_id", taskID), slog.Any("error", err))
	}
}
