# TASK-TM-03-04: Add `TerminalScrollbackSnapshotRepository` port

**From Solution:** SOL-TM-03
**Priority:** P0
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/usecase/ports.go`
**Depends on:** TASK-TM-03-02 (domain type)
**Status:** `[ ]` TODO

---

## Context

Defines the persistence port the usecases in this set (TASK-TM-03-05,
TASK-TM-03-06) depend on and the Postgres adapter (TASK-TM-03-07)
implements — parallel in shape to this file's existing
`TerminalSessionRepository`, tenant ID threaded explicitly on every method
per that port's own doc-comment rationale.

## Changes to make

Append to `backend-go/services/infra-fleet-service/internal/usecase/ports.go`
(near the existing `TerminalSessionRepository` interface):

```go
// TerminalScrollbackSnapshotRepository is the persistence port for
// infra.terminal_scrollback_snapshots (migrations/0009) — parallel in shape
// to TerminalSessionRepository, tenantID threaded explicitly on every
// method for the same reason that port's doc comment gives: an explicit
// parameter makes the tenant join impossible to forget at any
// implementation's call site.
type TerminalScrollbackSnapshotRepository interface {
	// Upsert writes or replaces the (tenantID, worktreeID, paneKey) row.
	Upsert(ctx context.Context, snap domain.TerminalScrollbackSnapshot) error
	// Get returns found=false, nil error when no snapshot exists yet for
	// this pane — mirrors ConnectionResolver's found-bool convention.
	Get(ctx context.Context, tenantID, worktreeID, paneKey string) (found bool, snap domain.TerminalScrollbackSnapshot, err error)
	// SumUncompressedBytes returns the current total across every pane for
	// worktreeID, EXCLUDING paneKey itself (the row Upsert is about to
	// replace) — backs BR-TM-10's per-worktree cap check.
	SumUncompressedBytes(ctx context.Context, tenantID, worktreeID, excludePaneKey string) (int64, error)
	// DeleteByWorktree removes every pane's snapshot for worktreeID — backs
	// git-gateway-service's RemoveWorktree cleanup hook.
	DeleteByWorktree(ctx context.Context, tenantID, worktreeID string) error
	// DeleteExpired removes every row with updated_at older than
	// domain.ScrollbackSnapshotTTL — backs BR-TM-12's sweep, called from a
	// scheduled job the same way fleet_health_samples' retention prune is.
	DeleteExpired(ctx context.Context, olderThan time.Time) (deletedCount int, err error)
}
```

Confirm `time` is already imported in `ports.go` (it is, per the existing
`Touch(ctx context.Context, tenantID, ptyID string, now time.Time) error`
method on `TerminalSessionRepository`) — no new import needed.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go vet ./services/infra-fleet-service/internal/usecase/...
```

Expected: clean build. This interface has no implementation yet
(TASK-TM-03-07 provides one) — nothing references it yet, so `go build`
succeeding on the interface declaration alone is sufficient at this step.
