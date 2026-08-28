# TASK-TM-03-02: Add `TerminalScrollbackSnapshot` domain type

**From Solution:** SOL-TM-03
**Priority:** P0
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/domain/terminal_scrollback_snapshot.go`
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

Adds the plain domain struct the repository port and usecases (later tasks
in this set) operate on, plus the two business-rule constants (BR-TM-10's
50MB per-worktree cap, BR-TM-12's 30-day TTL) — parallel in shape to this
service's existing `domain.TerminalSession`.

## Changes to make

Create `backend-go/services/infra-fleet-service/internal/domain/terminal_scrollback_snapshot.go`:

```go
package domain

import "time"

// TerminalScrollbackSnapshot is a durably-stored, client-serialized
// terminal buffer for one (worktree, pane) — survives across PTY respawns
// and app restarts, unlike TerminalSession, whose PtyID is dead by the time
// a snapshot is restored (a fresh PTY is spawned on reopen). DataGzip is
// opaque to this service (see SOL-TM-03's rationale) — an
// @xterm/addon-serialize ANSI blob, gzip-compressed.
type TerminalScrollbackSnapshot struct {
	TenantID          string
	WorktreeID        string
	PaneKey           string
	Cols, Rows        int32
	DataGzip          []byte
	UncompressedBytes int64
	LastTitle         string
	UpdatedAt         time.Time
}

// MaxSnapshotBytesPerWorktree enforces BR-TM-10 — 50MB per worktree, summed
// across every pane's UncompressedBytes.
const MaxSnapshotBytesPerWorktree int64 = 50 * 1024 * 1024

// ScrollbackSnapshotTTL enforces BR-TM-12. backend-go has no "worktree last
// opened" tracking (see SOL-TM-03's BR-TM-12 caveat), so this expires off
// the snapshot row's own updated_at instead — a pragmatic proxy for "not
// opened", not a literal implementation of the spec's wording.
const ScrollbackSnapshotTTL = 30 * 24 * time.Hour
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go vet ./services/infra-fleet-service/internal/domain/...
```

Expected: clean build, no vet warnings.
