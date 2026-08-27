package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

// GetTerminalScrollbackSnapshotResult carries GetTerminalScrollbackSnapshotResponse's fields.
type GetTerminalScrollbackSnapshotResult struct {
	Found      bool
	Cols, Rows int32
	Data       []byte // decompressed — the usecase, not the caller, owns ungzip
	LastTitle  string
	UpdatedAt  time.Time
}

type GetTerminalScrollbackSnapshot struct {
	snapshots TerminalScrollbackSnapshotRepository
}

func NewGetTerminalScrollbackSnapshot(snapshots TerminalScrollbackSnapshotRepository) *GetTerminalScrollbackSnapshot {
	return &GetTerminalScrollbackSnapshot{snapshots: snapshots}
}

func (uc *GetTerminalScrollbackSnapshot) Execute(ctx context.Context, worktreeID, paneKey string) (GetTerminalScrollbackSnapshotResult, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return GetTerminalScrollbackSnapshotResult{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	found, snap, err := uc.snapshots.Get(ctx, tenantID, worktreeID, paneKey)
	if err != nil {
		return GetTerminalScrollbackSnapshotResult{}, apperrors.New(apperrors.KindInternal, "INFRA_SCROLLBACK_GET_FAILED", "failed to load snapshot", err)
	}
	if !found {
		return GetTerminalScrollbackSnapshotResult{Found: false}, nil
	}
	data, err := gzipDecompress(snap.DataGzip)
	if err != nil {
		return GetTerminalScrollbackSnapshotResult{}, apperrors.New(apperrors.KindInternal, "INFRA_SCROLLBACK_DECOMPRESS_FAILED", "failed to decompress snapshot", err)
	}
	return GetTerminalScrollbackSnapshotResult{Found: true, Cols: snap.Cols, Rows: snap.Rows, Data: data, LastTitle: snap.LastTitle, UpdatedAt: snap.UpdatedAt}, nil
}
