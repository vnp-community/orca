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
