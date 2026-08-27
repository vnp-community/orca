# TASK-TM-03-03: Add scrollback-snapshot RPCs to `infrafleet.proto`

**From Solution:** SOL-TM-03
**Priority:** P0 — generated stubs from this task back every later task's compile
**Service:** `infra-fleet-service` (proto is shared, `backend-go/proto/`)
**File:** `backend-go/proto/orca/infrafleet/v1/infrafleet.proto`
**Depends on:** none
**Status:** `[x]` DONE — 3 RPCs + messages added, `buf generate` regenerated stubs cleanly, `go build ./proto/...` passes; `buf breaking` against `.git#branch=main` not runnable in this worktree (this repo's `main` predates `backend-go/` entirely — confirmed via `git ls-tree main`), verified additive-only by diff instead.

---

## Context

Adds the three new RPCs (`SaveTerminalScrollbackSnapshot`,
`GetTerminalScrollbackSnapshot`, `DeleteTerminalScrollbackSnapshots`) and
their messages to `InfraFleetService`. Additive only — no existing RPC or
field changes, so `buf breaking` stays clean.

## Changes to make

Add to the `InfraFleetService` service block in `infrafleet.proto`, near
the existing `SpawnTerminalSession`/terminal RPCs:

```protobuf
// --- Terminal scrollback persistence (SOL-TM-03) — distinct from
// AttachPty/live PTY I/O. NOT the same path as terminal.multiplex's
// SnapshotRequest opcode, which resolves against a LIVE pty_id this flow
// structurally cannot have (a fresh pty_id is spawned on worktree reopen).
rpc SaveTerminalScrollbackSnapshot(SaveTerminalScrollbackSnapshotRequest) returns (google.protobuf.Empty);
rpc GetTerminalScrollbackSnapshot(GetTerminalScrollbackSnapshotRequest) returns (GetTerminalScrollbackSnapshotResponse);
// Called by git-gateway-service's RemoveWorktree on hard worktree deletion
// — cleanup, not part of the save/restore flow itself.
rpc DeleteTerminalScrollbackSnapshots(DeleteTerminalScrollbackSnapshotsRequest) returns (google.protobuf.Empty);
```

Add messages (append near the existing `SpawnTerminalSessionRequest`/
`TerminalSession` messages):

```protobuf
message SaveTerminalScrollbackSnapshotRequest {
  string worktree_id = 1;
  string pane_key    = 2;
  int32  cols = 3;
  int32  rows = 4;
  bytes  data  = 5;   // raw ANSI text, NOT pre-gzipped by the caller — usecase compresses it
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

## Regenerate stubs

```bash
cd /opt/repos/orca/backend-go
buf generate proto
buf breaking proto --against '.git#branch=main'
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./proto/...
```

Expected: clean build, `buf breaking` reports no breaking changes (only
additions).
