package usecase

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// SaveTerminalScrollbackSnapshotInput mirrors the gRPC request 1:1 — see
// register_dev_server.go's comment for the rationale this service follows.
type SaveTerminalScrollbackSnapshotInput struct {
	WorktreeID string
	PaneKey    string
	Cols, Rows int32
	Data       []byte // raw ANSI text from the client, NOT yet gzipped
	LastTitle  string
}

// SaveTerminalScrollbackSnapshot enforces BR-TM-10's 50MB-per-worktree cap
// and persists a gzip-compressed, client-serialized terminal buffer.
type SaveTerminalScrollbackSnapshot struct {
	snapshots TerminalScrollbackSnapshotRepository
	clock     Clock
}

func NewSaveTerminalScrollbackSnapshot(snapshots TerminalScrollbackSnapshotRepository, clock Clock) *SaveTerminalScrollbackSnapshot {
	return &SaveTerminalScrollbackSnapshot{snapshots: snapshots, clock: clock}
}

func (uc *SaveTerminalScrollbackSnapshot) Execute(ctx context.Context, in SaveTerminalScrollbackSnapshotInput) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	if in.WorktreeID == "" || in.PaneKey == "" {
		return apperrors.New(apperrors.KindInvalidArgument, "INFRA_SCROLLBACK_MISSING_KEY", "worktreeId and paneKey are required", nil)
	}

	// BR-TM-10: reject rather than silently truncate — the client already
	// holds the full buffer and can retry with less scrollback; a silent
	// truncation here would corrupt BR-TM-11's "restore exactly" guarantee
	// for whatever was truncated.
	existingTotal, err := uc.snapshots.SumUncompressedBytes(ctx, tenantID, in.WorktreeID, in.PaneKey)
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "INFRA_SCROLLBACK_SUM_FAILED", "failed to sum existing snapshot bytes", err)
	}
	if existingTotal+int64(len(in.Data)) > domain.MaxSnapshotBytesPerWorktree {
		return apperrors.New(apperrors.KindFailedPrecondition, "INFRA_SCROLLBACK_OVER_CAP", "worktree scrollback snapshot cap (50MB) exceeded", nil)
	}

	compressed, err := gzipCompress(in.Data)
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "INFRA_SCROLLBACK_COMPRESS_FAILED", "failed to compress snapshot", err)
	}

	return uc.snapshots.Upsert(ctx, domain.TerminalScrollbackSnapshot{
		TenantID: tenantID, WorktreeID: in.WorktreeID, PaneKey: in.PaneKey,
		Cols: in.Cols, Rows: in.Rows, DataGzip: compressed,
		UncompressedBytes: int64(len(in.Data)), LastTitle: in.LastTitle,
		UpdatedAt: uc.clock.Now(),
	})
}

// gzipCompress/gzipDecompress are the stdlib compress/gzip helpers shared by
// SaveTerminalScrollbackSnapshot and GetTerminalScrollbackSnapshot
// (TASK-TM-03-06) — this service never inspects the decompressed content,
// only stores/returns it byte-for-byte (see SOL-TM-03's rationale).
func gzipCompress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func gzipDecompress(data []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}
