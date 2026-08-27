package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// ExpireTerminalScrollbackSnapshots backs BR-TM-12's daily sweep — invoked
// by a scheduled job, same "pruned by a scheduled job, not golang-migrate"
// pattern this service's fleet_health_samples retention prune already uses.
type ExpireTerminalScrollbackSnapshots struct {
	snapshots TerminalScrollbackSnapshotRepository
	clock     Clock
}

func NewExpireTerminalScrollbackSnapshots(snapshots TerminalScrollbackSnapshotRepository, clock Clock) *ExpireTerminalScrollbackSnapshots {
	return &ExpireTerminalScrollbackSnapshots{snapshots: snapshots, clock: clock}
}

func (uc *ExpireTerminalScrollbackSnapshots) Execute(ctx context.Context) (int, error) {
	cutoff := uc.clock.Now().Add(-domain.ScrollbackSnapshotTTL)
	deleted, err := uc.snapshots.DeleteExpired(ctx, cutoff)
	if err != nil {
		return 0, apperrors.New(apperrors.KindInternal, "INFRA_SCROLLBACK_EXPIRE_FAILED", "failed to expire scrollback snapshots", err)
	}
	return deleted, nil
}
