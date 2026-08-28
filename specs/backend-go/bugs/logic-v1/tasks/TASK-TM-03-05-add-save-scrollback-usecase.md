# TASK-TM-03-05: Add `SaveTerminalScrollbackSnapshot` usecase

**From Solution:** SOL-TM-03
**Priority:** P1
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/usecase/save_terminal_scrollback_snapshot.go`
**Depends on:** TASK-TM-03-04 (repository port)
**Status:** `[x]` DONE — save_terminal_scrollback_snapshot.go + test added (happy path, over-cap, missing-key); `go test -run TestSaveTerminalScrollbackSnapshot` — 3/3 pass.

---

## Context

The write path for scrollback snapshots: enforces BR-TM-10 (50MB
per-worktree cap, rejected explicitly rather than silently truncated —
truncating here would corrupt BR-TM-11's "restore exactly" guarantee),
gzip-compresses the client-supplied ANSI blob, and upserts it. Trusts the
caller's timing decision (BR-TM-09, "idle only") the same way
`SpawnTerminalSession` trusts the caller's `Shell` string — this
coordination-layer service never observes PTY bytes, so it cannot itself
detect idle.

## Changes to make

Create `backend-go/services/infra-fleet-service/internal/usecase/save_terminal_scrollback_snapshot.go`:

```go
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
```

Confirm this service already has a `Clock` port (used elsewhere for
testable time) — if not, add a minimal one to `ports.go`:

```go
// Clock abstracts time.Now for deterministic tests.
type Clock interface{ Now() time.Time }
```

and a `RealClock` implementation (`func (RealClock) Now() time.Time { return time.Now().UTC() }`)
wherever this service's other `Clock` usages already live — check first with
`grep -rn "Clock interface" backend-go/services/infra-fleet-service` before
adding a duplicate.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
```

Then add `save_terminal_scrollback_snapshot_test.go` with a fake
`TerminalScrollbackSnapshotRepository`:
- happy path upserts with gzip applied (decompress the captured `DataGzip`
  and assert it equals the original input)
- over-cap (`existingTotal + len(Data) > 50MB`) returns
  `INFRA_SCROLLBACK_OVER_CAP` without calling `Upsert`
- missing `WorktreeID`/`PaneKey` rejected before touching the repository

```bash
go test ./services/infra-fleet-service/internal/usecase/... -run TestSaveTerminalScrollbackSnapshot -v
```

Expected: clean build, all three cases pass.
