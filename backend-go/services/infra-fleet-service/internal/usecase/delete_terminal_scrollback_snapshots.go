package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

// DeleteTerminalScrollbackSnapshots removes every pane's snapshot for one
// worktree — called by git-gateway-service's RemoveWorktree cleanup hook on
// hard worktree deletion, best-effort (see that call site's doc comment).
type DeleteTerminalScrollbackSnapshots struct {
	snapshots TerminalScrollbackSnapshotRepository
}

func NewDeleteTerminalScrollbackSnapshots(snapshots TerminalScrollbackSnapshotRepository) *DeleteTerminalScrollbackSnapshots {
	return &DeleteTerminalScrollbackSnapshots{snapshots: snapshots}
}

func (uc *DeleteTerminalScrollbackSnapshots) Execute(ctx context.Context, worktreeID string) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	if err := uc.snapshots.DeleteByWorktree(ctx, tenantID, worktreeID); err != nil {
		return apperrors.New(apperrors.KindInternal, "INFRA_SCROLLBACK_DELETE_FAILED", "failed to delete worktree's scrollback snapshots", err)
	}
	return nil
}
