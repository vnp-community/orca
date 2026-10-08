package usecase

import (
	"context"
	"log/slog"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// reconcileBatch bounds containers repaired per sweep.
const reconcileBatch = 50

// ContainerReconcileRepository finds containers that a child change may have left stale.
type ContainerReconcileRepository interface {
	// ListStaleContainers returns non-cancelled plan/phase rows whose updated_at is older
	// than their newest child's, across tenants.
	ListStaleContainers(ctx context.Context, limit int) ([]domain.Task, error)
	// TouchContainer bumps updated_at so a checked-and-unchanged container is not rescanned.
	TouchContainer(ctx context.Context, tenantID, id string) error
}

// ReconcileContainerStatuses is the safety net for container syncs that were skipped or lost:
// bulk child changes such as ReleaseUnlinkedInProgress never name the tasks they moved.
type ReconcileContainerStatuses struct {
	repo ContainerReconcileRepository
	sync *SyncContainerStatus
}

func NewReconcileContainerStatuses(repo ContainerReconcileRepository, sync *SyncContainerStatus) *ReconcileContainerStatuses {
	return &ReconcileContainerStatuses{repo: repo, sync: sync}
}

// Execute returns how many containers had their status corrected.
func (uc *ReconcileContainerStatuses) Execute(ctx context.Context) (int, error) {
	stale, err := uc.repo.ListStaleContainers(ctx, reconcileBatch)
	if err != nil {
		return 0, err
	}
	fixed := 0
	for _, c := range stale {
		tctx := tenant.WithTenantID(ctx, c.TenantID)
		changed, err := uc.sync.SyncOne(tctx, c.ID)
		if err != nil {
			slog.WarnContext(ctx, "task: container reconcile failed", slog.String("task_id", c.ID), slog.Any("error", err))
			continue
		}
		if changed {
			fixed++
			slog.WarnContext(ctx, "task: container status reconciled", slog.String("task_id", c.ID))
			continue
		}
		if err := uc.repo.TouchContainer(tctx, c.TenantID, c.ID); err != nil {
			slog.WarnContext(ctx, "task: container touch failed", slog.String("task_id", c.ID), slog.Any("error", err))
		}
	}
	return fixed, nil
}
