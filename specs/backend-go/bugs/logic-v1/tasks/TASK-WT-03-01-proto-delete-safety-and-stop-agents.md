# TASK-WT-03-01: Proto — `CheckWorktreeDeleteSafety` RPC, `RemoveWorktree` gains `stop_agents` and a real response

**From Solution:** SOL-WT-03
**Priority:** P0 — every other task in this set depends on these wire types
**Service:** `git-gateway-service`
**File:** `backend-go/proto/orca/gitgateway/v1/gitgateway.proto`
**Depends on:** none
**Status:** `[x]` DONE — Added CheckWorktreeDeleteSafety RPC + messages; RemoveWorktreeRequest gained stop_agents, RemoveWorktreeResponse replaces google.protobuf.Empty. buf generate + go build ./proto/... clean.

---

## Context

[SOL-WT-03](../solutions/SOL-WT-03-xoa-worktree.md) closes [BUG-WT-03](../BUG-WT-03-xoa-worktree-partial.md): today `RemoveWorktree`'s only safety net is git's own dirty-worktree refusal, fully bypassed by `force=true` (`RemoveWorktreeRequest` currently has only `worktree_id`/`force`, verified at `gitgateway.proto:629-632`), and `RemoveWorktree` returns `google.protobuf.Empty` (`gitgateway.proto:87`) so there's no data shape for a client's [A1]/[A2] recovery dialogs. This is a breaking wire change to `RemoveWorktree`'s return type — acceptable because it has exactly one caller today (`wscompat`'s `worktree.rm`, updated in [TASK-WT-03-06](./TASK-WT-03-06-wscompat-channels.md)).

## Changes to make

In `backend-go/proto/orca/gitgateway/v1/gitgateway.proto`, change the RPC declaration (around line 87):

```protobuf
  rpc RemoveWorktree(RemoveWorktreeRequest) returns (RemoveWorktreeResponse);
  // NEW — read-only pre-delete check, called by the client before rendering
  // the confirm dialog (mirrors worktree.detectedList's separate-read-call
  // shape).
  rpc CheckWorktreeDeleteSafety(CheckWorktreeDeleteSafetyRequest) returns (CheckWorktreeDeleteSafetyResponse);
```

Replace `RemoveWorktreeRequest` and add the new messages (near `gitgateway.proto:629`):

```protobuf
message CheckWorktreeDeleteSafetyRequest { string worktree_id = 1; }
message CheckWorktreeDeleteSafetyResponse {
  int32 uncommitted_files = 1;   // modified + added + deleted + conflicted, per FileStatus.state
  int32 untracked_files = 2;
  bool agent_running = 3;        // heuristic: any active PTY session whose cwd is under this worktree's path — see SOL-WT-03's rationale for why this isn't a precise "is this an agent" check
  repeated string active_pty_ids = 4;
  bool safe_to_delete = 5;       // true iff all counts are zero and !agent_running
}

message RemoveWorktreeRequest {
  string worktree_id = 1;
  bool   force = 2;        // maps to `git worktree remove --force` (uncommitted changes present)
  bool   stop_agents = 3;  // NEW — spec's "Stop & Delete" choice: kill active PTY sessions found in this worktree before removing it
}
message RemoveWorktreeResponse {  // NEW — replaces google.protobuf.Empty
  int32 uncommitted_files_discarded = 1; // echoes the safety-check count that was overridden by force, for the UI's post-delete confirmation toast
  repeated string stopped_pty_ids = 2;
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
buf generate proto
buf breaking proto --against '.git#branch=main'
go build ./proto/...
```

Expected: `buf breaking` reports `RemoveWorktree`'s return-type change as a breaking change — this is expected and acceptable (documented above); confirm no OTHER unintended breaking change appears in the diff. `go build ./proto/...` must still be clean.
