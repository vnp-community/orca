# TASK-TM-03-06: Add Get/Delete/Expire scrollback usecases

**From Solution:** SOL-TM-03
**Priority:** P1
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/usecase/get_terminal_scrollback_snapshot.go` (+ `delete_terminal_scrollback_snapshots.go`, `expire_terminal_scrollback_snapshots.go`)
**Depends on:** TASK-TM-03-04 (repository port), TASK-TM-03-05 (gzip helpers)
**Status:** `[x]` DONE — Get/Delete/Expire usecases + tests added; `go test -run "TestGetTerminalScrollbackSnapshot|TestDeleteTerminalScrollbackSnapshots|TestExpireTerminalScrollbackSnapshots"` — 5/5 pass.

---

## Context

The read/cleanup side of scrollback persistence: `Get` decompresses and
returns a snapshot (found=false, nil error on a never-saved pane — mirrors
`ConnectionResolver`'s found-bool convention, not a NotFound error);
`DeleteByWorktree`'s wrapper backs git-gateway-service's `RemoveWorktree`
cleanup hook; `DeleteExpired`'s wrapper backs BR-TM-12's daily sweep.

## Changes to make

Create `backend-go/services/infra-fleet-service/internal/usecase/get_terminal_scrollback_snapshot.go`:

```go
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
```

Create `backend-go/services/infra-fleet-service/internal/usecase/delete_terminal_scrollback_snapshots.go`:

```go
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
```

Create `backend-go/services/infra-fleet-service/internal/usecase/expire_terminal_scrollback_snapshots.go`:

```go
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
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
```

Then add tests with a fake `TerminalScrollbackSnapshotRepository`:
- `get_terminal_scrollback_snapshot_test.go`: found=false round-trips as
  `Found: false` with no error; a stored gzip blob decompresses back to the
  exact original bytes (BR-TM-11 round-trip fidelity regression guard).
- `delete_terminal_scrollback_snapshots_test.go`: deletes every pane's row
  for the given worktree, leaves other worktrees untouched (assert via the
  fake's recorded call args).
- `expire_terminal_scrollback_snapshots_test.go`: rows older than 30 days
  deleted; rows newer than 30 days survive; asserts `DeleteExpired` is
  called with `clock.Now() - 30d`.

```bash
go test ./services/infra-fleet-service/internal/usecase/... -run "TestGetTerminalScrollbackSnapshot|TestDeleteTerminalScrollbackSnapshots|TestExpireTerminalScrollbackSnapshots" -v
```

Expected: clean build, all cases pass.
