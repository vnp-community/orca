# SOL-TM-03: Persist and restore terminal scrollback across worktree close/reopen

**Resolves:** [BUG-TM-03](../BUG-TM-03-scrollback-persistence-not-implemented.md)
**Service:** `infra-fleet-service` (extended) + `api-gateway`
**Affected files (proposed):**
- `backend-go/services/infra-fleet-service/migrations/0009_terminal_scrollback_snapshots.up.sql` (+ `.down.sql`)
- `backend-go/services/infra-fleet-service/internal/domain/terminal_scrollback_snapshot.go`
- `backend-go/services/infra-fleet-service/internal/usecase/ports.go` (extend: `TerminalScrollbackSnapshotRepository`)
- `backend-go/services/infra-fleet-service/internal/usecase/save_terminal_scrollback_snapshot.go`, `get_terminal_scrollback_snapshot.go`, `delete_terminal_scrollback_snapshots.go`, `expire_terminal_scrollback_snapshots.go`
- `backend-go/services/infra-fleet-service/internal/adapter/postgres/terminal_scrollback_snapshot_repository.go`
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto` (new RPCs)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_terminal_scrollback.go` (new)
- `backend-go/services/git-gateway-service/internal/adapter/grpc/server.go` (`RemoveWorktree` — cleanup hook)
- `backend-go/services/infra-fleet-service/internal/usecase/*_test.go`, `postgres/terminal_scrollback_snapshot_repository_test.go`, `wscompat/channels_terminal_scrollback_test.go`
**Status:** 📋 Proposed — not yet implemented

---

## Design rationale (grounded in TDD)

### Where this belongs: `infra-fleet-service`, extending its existing terminal domain

`infra-fleet-service.md` §1 already states this service owns "Terminal/PTY
session **routing**... Folded into this service... because it is the
identical 'which host owns this `connectionId`' logic as everything else
here" (`infra-fleet-service.md:28-33`), and §4 already lists `TerminalSession`
as one of this service's domain objects, backed by its own `infra` schema
(§5). A scrollback snapshot is worktree/pane-scoped state that outlives any
one `TerminalSession` row (the `pty_id` a snapshot was captured from is dead
by the time it's restored — a brand-new `pty_id` is spawned on reopen), but
it is still squarely "terminal state this service is the system of record
for" — no other service has a terminal-shaped table, and per
[`05-data-architecture.md`](../../../tdd/architecture/05-data-architecture.md)'s
"no cross-database queries... a service that needs data another service
owns calls that service's API" rule, bolting scrollback storage onto
`project-service` (which owns `worktree_id` itself, per its `internal/domain/worktree.go`)
would require `project-service` to reach into terminal semantics it has no
other reason to know about. Extending `infra-fleet-service`'s own `infra`
schema (already extended once beyond ADR-021's original two tables per
§5's explicit note, `infra-fleet-service.md:182-188`) is the smaller,
consistent move — one more deliberate, documented extension in the same
place the prior ones live.

### The serialize/deserialize step is a client concern, not a backend one

BL-TM-03 §"Luồng Serialize" step 2a names `@xterm/addon-serialize` — an
xterm.js (renderer-side) library — explicitly. This matches
`infra-fleet-service.md`'s own bounded-context table (§2, `infra-fleet-service.md:62-70`):
"PTY byte I/O (the actual terminal data stream) | No — routes the request
to the right connection, **does not touch the bytes**". A scrollback
snapshot is exactly PTY byte content (an ANSI-encoded terminal-buffer
image, cursor position and text attributes included inline in that ANSI
stream by construction of the xterm.js serializer format) — parsing or
generating it server-side would put backend-go in the business of
understanding terminal escape sequences, which §2's table draws the line
against for every other PTY-adjacent RPC this service already has
(`SpawnTerminalSession`, `AttachPty`, `ResizeTerminalSession`). Confirmed by
the prior system's own architecture: `frontend/src/renderer/src/components/terminal-pane/pty-buffer-serializer.ts`
is the renderer-side registry that answers a serialize request by calling
into the live xterm.js `Terminal` instance in-process — there is no
equivalent server-side terminal emulator in backend-go's design (the one
analogous component, `backend/src/main/daemon/headless-emulator.ts`, is Electron-main-only and belongs to a
*different* mechanism — see "Two distinct snapshot mechanisms" below — not
this one).

Backend-go's job is therefore a **storage surface only**: accept an
already-serialized (ANSI text), already-cursor/attribute-encoded blob from
the client, gzip-compress and persist it (BR-TM-10's cap, BR-TM-12's
expiry), and hand the same blob back unmodified on request. This mirrors
[`SOL-009`](../../missing-v1/solutions/SOL-009-files-channels.md)'s framing
of `files.*`: content the backend stores and moves but does not interpret.

### Two distinct snapshot mechanisms — do not conflate with `terminal.multiplex`'s `SnapshotRequest`

`BUG-TM-03` cites `channels_terminal_multiplex.go:14-28`'s documented
no-op for `SnapshotRequest`/`SnapshotStart`/`Chunk`/`End`. Reading the real
system's handler this opcode maps to
(`backend/src/main/runtime/rpc/methods/terminal.ts:1891-1940`,
`561-617`) shows it calls `runtime.serializeTerminalBuffer(stream.ptyId, ...)`
— it serializes the **live, currently-attached** PTY's current screen, for
mobile/remote-desktop reconnect ("get me what's on screen right now"). That
mechanism structurally cannot serve BL-TM-03's use case: the `pty_id` it
needs is gone once the app/worktree has actually closed and reopened (a
fresh PTY is spawned on reopen, per `SpawnTerminalSession`'s own doc
comment, `infra-fleet-service/internal/usecase/spawn_terminal_session.go:22-36`).
This solution's flow is a **distinct, durable, worktree-keyed** mechanism —
closer to the old system's `terminal-scrollback-snapshots.ts`
(`backend/src/main/terminal-scrollback-snapshots.ts`), which persists a
renderer-produced ANSI blob to disk keyed by a stable per-pane identifier
independent of any live `ptyId`, and is read back into a *newly created*
xterm.js instance on reopen — not into a live PTY at all. Leaving
`terminal.multiplex`'s `SnapshotRequest` a no-op remains correct and is
**not** touched by this solution; conflating the two would misroute a
live-reconnect request into a stale, possibly-30-days-old blob. Flagged
explicitly since `BUG-TM-03`'s own text could be read as "wire up
`SnapshotRequest`" — that is the wrong fix.

### Keying: `(tenant_id, worktree_id, pane_key)`, not `worktree_id` alone

BL-TM-03's literal text says "keyed by worktree_id", but F02 (Terminal
Splits — this BL's own `Tính năng` field) means one worktree can have
multiple simultaneous panes, each needing its own scrollback. The old
system's actual key was `sha256(tabId, leafId)`
(`backend/src/main/ipc/pty-pane-key-registry.ts:1-11`'s `parsePaneKey`,
`terminal-scrollback-snapshots.ts:41-44`'s `makeTerminalScrollbackSnapshotRef`)
— a stable per-pane identifier, not the worktree alone. backend-go's
closest equivalent already exists on the wire: the agent's own PTY-spawn
env already threads `ORCA_PANE_KEY`/`ORCA_WORKTREE_ID` through as opaque
strings the relay treats as pass-through (`agent/src/relay/pty-handler.ts:679-753`,
and `TerminalSideEffectBatch`'s `worktreeId`/`tabId`/`paneKey` fields,
`agent/src/shared/terminal-side-effect-facts.ts:44-48`). This solution's
schema uses `(tenant_id, worktree_id, pane_key)` as the natural key,
flagged as a deliberate refinement of BL-TM-03's literal wording rather
than a literal transcription — same shape SOL-009 used when it collapsed
`createDirNoClobber`/`writeBase64` into one RPC each, an explicit,
documented departure from the spec's literal 1:1 phrasing, not an omission.

### BR-TM-09 ("idle only") is enforced client-side, structurally

Per the bounded-context table cited above, backend-go cannot itself detect
"idle" — it never observes PTY bytes to know whether output is actively
flowing. The client (which already owns the xterm.js instance and the
serialize call) decides *when* to call `Save` — typically debounced off its
own last-output-observed timestamp, mirroring how the old system's
`terminal-shutdown-layout-capture.ts` only fires at pane-teardown/app-close
time, never mid-stream. This usecase's `Save` trusts that timing decision
the same way `usecase.SpawnTerminalSession` trusts the caller's `Shell`
string — validating it is out of scope for a coordination-layer service.

---

## Design — schema

```sql
-- backend-go/services/infra-fleet-service/migrations/0009_terminal_scrollback_snapshots.up.sql
--
-- One row per (tenant, worktree, pane) — see this file's companion
-- solution doc for why pane_key, not worktree_id alone, is the key.
-- data_gzip is the CLIENT-produced @xterm/addon-serialize ANSI blob,
-- gzip-compressed; this service never parses it (see "rationale" above).
-- Cursor position/text attributes (BR-TM-11) are encoded INLINE in that
-- ANSI blob by the serializer itself — no separate cursor_row/cursor_col
-- columns; extracting them here would require this service to understand
-- terminal escape sequences, which it deliberately does not.
CREATE TABLE infra.terminal_scrollback_snapshots (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL,
    worktree_id         UUID NOT NULL,
    pane_key            TEXT NOT NULL,
    cols                INTEGER NOT NULL,
    rows                INTEGER NOT NULL,
    data_gzip           BYTEA NOT NULL,
    uncompressed_bytes  INTEGER NOT NULL,  -- pre-compression size, for BR-TM-10's cap check
    last_title          TEXT NOT NULL DEFAULT '',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, worktree_id, pane_key)
);

CREATE INDEX idx_infra_scrollback_snapshots_worktree
    ON infra.terminal_scrollback_snapshots (tenant_id, worktree_id);
-- Backs BR-TM-12's expiry sweep — see ExpireTerminalScrollbackSnapshots below.
CREATE INDEX idx_infra_scrollback_snapshots_updated_at
    ON infra.terminal_scrollback_snapshots (updated_at);

ALTER TABLE infra.terminal_scrollback_snapshots ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON infra.terminal_scrollback_snapshots
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
```

**BR-TM-12 caveat, flagged explicitly**: the spec says snapshots expire
"30 days if the worktree is not opened." backend-go's `project-service`
worktree domain (`backend-go/services/project-service/internal/domain/worktree.go`)
has no "last opened at" tracking today — only creation/mutation
timestamps tied to git operations, not UI-visit events. Building that
tracking is a `project-service`-side feature this bug does not otherwise
need. This solution instead expires off `updated_at` on the snapshot row
itself — i.e., "30 days since this pane's scrollback was last *saved*",
which happens once per close and is a reasonable proxy for "not opened"
(a worktree that keeps getting opened and closed keeps refreshing its
snapshot's `updated_at`; one that's abandoned stops). This is a
pragmatic v1 substitution, not a literal implementation of the spec's
wording — call it out for review; true "worktree last-opened" tracking is
a larger, separate `project-service` change if the distinction ever
matters (e.g., a worktree opened read-only without touching any terminal
pane would not refresh this table under either definition, so the
practical difference is narrow).

## Design — domain

```go
// internal/domain/terminal_scrollback_snapshot.go
package domain

import "time"

// TerminalScrollbackSnapshot is a durably-stored, client-serialized
// terminal buffer for one (worktree, pane) — survives across PTY
// respawns and app restarts, unlike TerminalSession. DataGzip is opaque
// to this service (see SOL-TM-03's rationale) — an @xterm/addon-serialize
// ANSI blob, gzip-compressed.
type TerminalScrollbackSnapshot struct {
	TenantID           string
	WorktreeID          string
	PaneKey             string
	Cols, Rows          int32
	DataGzip            []byte
	UncompressedBytes   int64
	LastTitle           string
	UpdatedAt           time.Time
}

// MaxSnapshotBytesPerWorktree enforces BR-TM-10 — 50MB per worktree,
// summed across every pane's UncompressedBytes.
const MaxSnapshotBytesPerWorktree int64 = 50 * 1024 * 1024

// ScrollbackSnapshotTTL enforces BR-TM-12 (see this solution's schema
// section for the updated_at-based proxy this uses for "not opened").
const ScrollbackSnapshotTTL = 30 * 24 * time.Hour
```

## Design — ports (`usecase/ports.go`, extended)

```go
// TerminalScrollbackSnapshotRepository is the persistence port for
// infra.terminal_scrollback_snapshots — parallel in shape to
// TerminalSessionRepository, tenantID threaded explicitly on every method
// per that port's own doc comment rationale.
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
	// the RemoveWorktree cleanup hook below.
	DeleteByWorktree(ctx context.Context, tenantID, worktreeID string) error
	// DeleteExpired removes every row with updated_at older than
	// domain.ScrollbackSnapshotTTL — backs BR-TM-12's sweep, called from a
	// scheduled job the same way fleet_health_samples' retention prune is
	// (infra-fleet-service.md §5's "pruned by a scheduled job, not
	// golang-migrate" note, `infra-fleet-service.md:256-258`).
	DeleteExpired(ctx context.Context, olderThan time.Time) (deletedCount int, err error)
}
```

## Design — usecases

```go
// internal/usecase/save_terminal_scrollback_snapshot.go
type SaveTerminalScrollbackSnapshotInput struct {
	WorktreeID string
	PaneKey    string
	Cols, Rows int32
	Data       []byte // raw ANSI text from the client, NOT yet gzipped
	LastTitle  string
}

type SaveTerminalScrollbackSnapshot struct {
	snapshots TerminalScrollbackSnapshotRepository
	clock     Clock
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
	// holds the full buffer and can decide to retry with less scrollback;
	// a silent truncation here would corrupt BR-TM-11's "restore exactly"
	// guarantee for whatever WAS truncated. Mirrors SOL-009's "explicit
	// error over silent drop" precedent.
	existingTotal, err := uc.snapshots.SumUncompressedBytes(ctx, tenantID, in.WorktreeID, in.PaneKey)
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "INFRA_SCROLLBACK_SUM_FAILED", "failed to sum existing snapshot bytes", err)
	}
	if existingTotal+int64(len(in.Data)) > domain.MaxSnapshotBytesPerWorktree {
		return apperrors.New(apperrors.KindFailedPrecondition, "INFRA_SCROLLBACK_OVER_CAP", "worktree scrollback snapshot cap (50MB) exceeded", nil)
	}

	compressed, err := gzipCompress(in.Data) // internal helper, stdlib compress/gzip
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
```

```go
// internal/usecase/get_terminal_scrollback_snapshot.go
type GetTerminalScrollbackSnapshotResult struct {
	Found      bool
	Cols, Rows int32
	Data       []byte // decompressed — the usecase, not the caller, owns ungzip
	LastTitle  string
	UpdatedAt  time.Time
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

`DeleteTerminalScrollbackSnapshots(worktreeID)` and
`ExpireTerminalScrollbackSnapshots()` usecases are thin one-line wrappers
over the repository's `DeleteByWorktree`/`DeleteExpired` — same shape as
every other simple usecase in this service, omitted here for brevity.

## Design — proto (`orca.infrafleet.v1`, extends `infrafleet.proto`)

```protobuf
service InfraFleetService {
  // ... existing RPCs unchanged ...

  // --- Terminal scrollback persistence (SOL-TM-03) — distinct from
  // AttachPty/live PTY I/O; see this solution's "Two distinct snapshot
  // mechanisms" rationale for why this is NOT the same path as
  // terminal.multiplex's SnapshotRequest opcode. ---
  rpc SaveTerminalScrollbackSnapshot(SaveTerminalScrollbackSnapshotRequest) returns (google.protobuf.Empty);
  rpc GetTerminalScrollbackSnapshot(GetTerminalScrollbackSnapshotRequest) returns (GetTerminalScrollbackSnapshotResponse);
  // Called by git-gateway-service's RemoveWorktree on hard worktree
  // deletion — cleanup, not part of the save/restore flow itself.
  rpc DeleteTerminalScrollbackSnapshots(DeleteTerminalScrollbackSnapshotsRequest) returns (google.protobuf.Empty);
}

message SaveTerminalScrollbackSnapshotRequest {
  string worktree_id = 1;
  string pane_key    = 2;
  int32  cols = 3;
  int32  rows = 4;
  bytes  data  = 5;   // raw ANSI text, NOT pre-gzipped by the caller — see usecase doc comment
  string last_title = 6;
}

message GetTerminalScrollbackSnapshotRequest {
  string worktree_id = 1;
  string pane_key    = 2;
}
message GetTerminalScrollbackSnapshotResponse {
  bool   found = 1;
  int32  cols = 2;
  int32  rows = 3;
  bytes  data = 4;      // decompressed
  string last_title = 5;
  int64  updated_at_unix_ms = 6;
}

message DeleteTerminalScrollbackSnapshotsRequest {
  string worktree_id = 1;
}
```

## Design — wiring

### `wscompat`: two new channels, not an extension of `terminal.multiplex`

```go
// channels_terminal_scrollback.go
//
// terminal.scrollback.save / terminal.scrollback.restore — deliberately
// NOT part of terminal.multiplex's opcode set (see this solution's "Two
// distinct snapshot mechanisms" rationale: that protocol's SnapshotRequest
// resolves against a LIVE ptyId this flow structurally cannot have).
// Plain JSON channels, matching terminal.create/terminal.list's shape,
// not terminal.multiplex's binary framing — there is no low-latency
// requirement here (this fires once per pane teardown/restore, not per
// keystroke).

type scrollbackSaveArgs struct {
	WorktreeID string `json:"worktreeId"`
	PaneKey    string `json:"paneKey"`
	Cols       int32  `json:"cols"`
	Rows       int32  `json:"rows"`
	Data       string `json:"data"` // xterm SerializeAddon output, UTF-8 text
	LastTitle  string `json:"lastTitle"`
}

func registerTerminalScrollbackChannels(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
	r.Register("terminal.scrollback.save", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := decodeArg[scrollbackSaveArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		_, err = client.SaveTerminalScrollbackSnapshot(ctx, &infrafleetv1.SaveTerminalScrollbackSnapshotRequest{
			WorktreeId: in.WorktreeID, PaneKey: in.PaneKey, Cols: in.Cols, Rows: in.Rows,
			Data: []byte(in.Data), LastTitle: in.LastTitle,
		})
		return map[string]bool{"ok": err == nil}, err
	})

	r.Register("terminal.scrollback.restore", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := decodeArg[struct {
			WorktreeID string `json:"worktreeId"`
			PaneKey    string `json:"paneKey"`
		}](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		resp, err := client.GetTerminalScrollbackSnapshot(ctx, &infrafleetv1.GetTerminalScrollbackSnapshotRequest{WorktreeId: in.WorktreeID, PaneKey: in.PaneKey})
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"found": resp.GetFound(), "cols": resp.GetCols(), "rows": resp.GetRows(),
			"data": string(resp.GetData()), "lastTitle": resp.GetLastTitle(),
			"updatedAt": resp.GetUpdatedAtUnixMs(),
		}, nil
	})
}
```

`RegisterRealChannels` (channels.go) gains
`registerTerminalScrollbackChannels(r, infraClient)` next to the existing
`registerTerminalChannels(r, infraClient)` call.

### Worktree-removal cleanup hook

`git-gateway-service`'s `RemoveWorktree`
(`backend-go/services/git-gateway-service/internal/adapter/grpc/server.go:696-701`)
gains a best-effort call to `infra-fleet-service`'s new
`DeleteTerminalScrollbackSnapshots(worktreeId)` after the worktree itself
is removed — mirrors the existing `git --> infra` dependency edge already
in `02-microservices-decomposition.md`'s dependency graph (no new edge
needed), and prevents orphaned snapshot rows for a worktree that no longer
exists from silently surviving to the BR-TM-12 sweep 30 days later. Failure
to clean up is logged, not surfaced as a `RemoveWorktree` error — the
worktree removal itself must not fail because a housekeeping call to a
different service failed, matching this service's existing best-effort
posture toward secondary side effects elsewhere in `RemoveWorktree`.

### BR-TM-12 sweep

A scheduled job (cron-triggered `usecase.ExpireTerminalScrollbackSnapshots`
invocation) runs daily, calling `DeleteExpired(now - 30d)` — same "pruned
by a scheduled job, not golang-migrate" pattern
`infra-fleet-service.md:256-258` already specifies for
`fleet_health_samples`' retention.

---

## Test plan

- `usecase/save_terminal_scrollback_snapshot_test.go` — fake repository:
  happy path upserts with gzip applied; over-cap (existing total + new >
  50MB) returns `INFRA_SCROLLBACK_OVER_CAP` without calling `Upsert`;
  missing `worktreeId`/`paneKey` rejected before touching the repository.
- `usecase/get_terminal_scrollback_snapshot_test.go` — found=false round
  trips as `Found: false` with no error (not a NotFound error, matching
  `ConnectionResolver`'s convention); a stored gzip blob decompresses back
  to the exact original bytes (regression guard for BR-TM-11 — round-trip
  fidelity, since this service never inspects the content, only that it
  survives byte-for-byte).
- `usecase/delete_terminal_scrollback_snapshots_test.go` — deletes every
  pane's row for a worktree, leaves other worktrees' rows untouched.
- `usecase/expire_terminal_scrollback_snapshots_test.go` — rows older than
  30 days deleted, rows newer than 30 days (including one saved yesterday
  for an otherwise-ancient worktree) survive.
- `adapter/postgres/terminal_scrollback_snapshot_repository_test.go` —
  `testcontainers-go` Postgres: `SumUncompressedBytes` excludes the pane
  being replaced (two saves to the same pane never double-count toward the
  cap); RLS policy blocks a cross-tenant `Get`.
- `wscompat/channels_terminal_scrollback_test.go` — `terminal.scrollback.save`
  round-trips through a fake `InfraFleetServiceClient`; `terminal.scrollback.restore`
  on a never-saved pane returns `{found: false}`, not an error.
- Integration: `git-gateway-service`'s `RemoveWorktree` test gains a case
  asserting the cleanup RPC is called with the removed worktree's ID, and
  that a cleanup RPC failure does not fail `RemoveWorktree` itself.

## References

- `docs/logic/terminal-management/BL-TM-03-scrollback-persistence.md` — spec text, BR-TM-09..12
- `specs/backend-go/tdd/services/infra-fleet-service.md:12-38` (§1, terminal domain ownership rationale), `:53-76` (§2, bounded-context "does not touch the bytes" table), `:140-166` (§4, `TerminalSession` domain shape), `:174-282` (§5, `infra` schema + ADR-021 extension precedent), `:244-258` (`fleet_health_samples` scheduled-prune precedent this solution's BR-TM-12 sweep follows)
- `specs/backend-go/tdd/architecture/05-data-architecture.md:3-25` (database-per-service, no cross-database queries — why this doesn't live in `project-service`)
- `specs/backend-go/tdd/architecture/03-clean-architecture-guidelines.md:73-89` (usecase-defines-ports convention this solution's `TerminalScrollbackSnapshotRepository` follows)
- `specs/backend-go/bugs/missing-v1/solutions/SOL-009-files-channels.md` (explicit-error-over-silent-drop precedent for BR-TM-10's cap enforcement)
- `backend-go/services/infra-fleet-service/internal/domain/terminal_session.go:1-25`, `internal/usecase/spawn_terminal_session.go:22-36` (why a persisted `pty_id` can't be the restore key)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_terminal_multiplex.go:14-40` (the `SnapshotRequest` no-op this solution deliberately leaves alone)
- `backend/src/main/runtime/rpc/methods/terminal.ts:561-617,1891-1940` (real system's `SnapshotRequest` handler — confirmed live-`ptyId`-bound, not this flow)
- `backend/src/main/terminal-scrollback-snapshots.ts:1-172` (prior system's closest analog — disk-file storage keyed by `sha256(tabId, leafId)`, gzip/compression boundary this solution mirrors in Postgres form)
- `backend/src/main/ipc/pty-pane-key-registry.ts:1-30` (`paneKey` as the stable per-pane identifier this solution's key follows)
- `frontend/src/renderer/src/components/terminal-pane/pty-buffer-serializer.ts:1-25,113-156` (confirms the serialize call is renderer-side, in-process against a live xterm.js `Terminal`)
- `agent/src/shared/terminal-side-effect-facts.ts:44-48` (`TerminalSideEffectBatch`'s `worktreeId`/`paneKey` fields — same pane-addressing convention this solution's key reuses)
- `backend-go/services/git-gateway-service/internal/adapter/grpc/server.go:696-701` (`RemoveWorktree` — cleanup hook site)
