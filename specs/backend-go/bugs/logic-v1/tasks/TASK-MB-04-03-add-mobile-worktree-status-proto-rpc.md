# TASK-MB-04-03: Add `GetMobileWorktreeStatus` RPC to `project.proto`

**From Solution:** SOL-MB-04
**Priority:** P0
**Service:** `project-service`
**File:** `backend-go/proto/orca/project/v1/project.proto`
**Depends on:** TASK-MB-04-01
**Status:** `[ ]` TODO

---

## Context

`api-gateway.md` §2 forbids cross-service response orchestration in the
gateway — the composed read (worktree identity + PTY/agent runtime status)
belongs to `project-service`, which already has a dependency edge to
`infra-fleet-service` (validates `devServerId` bindings). This is the ONE
composed-read call BL-MB-04's response shape reduces to. Additive-only.

## Changes to make

Add to the `ProjectService` service block:

```protobuf
// GetMobileWorktreeStatus is the ONE composed-read call BL-MB-04 reduces
// to — project-service already depends on infra-fleet-service (dev-server
// binding validation), so this extends that existing edge rather than
// adding a new cross-service dependency.
rpc GetMobileWorktreeStatus(GetMobileWorktreeStatusRequest) returns (GetMobileWorktreeStatusResponse);
```

Add messages (append to the bottom of the file):

```protobuf
message GetMobileWorktreeStatusRequest {} // tenant_id/user_id via metadata

message GetMobileWorktreeStatusResponse {
  repeated MobileWorktreeStatus worktrees = 1;
  int64 generated_at_unix_ms = 2; // BR-MB-16: client computes "last updated X ago" from this
}

message MobileWorktreeStatus {
  string id = 1;              // Worktree.ID
  string name = 2;            // Worktree.Branch
  string agent = 3;           // AgentKind, e.g. "claude" | "codex" | "" if none running
  string status = 4;          // "completed" | "running" | "waiting" | "idle" | "unknown"
  int64  duration_ms = 5;     // now - TerminalSession.CreatedAt for a running session; 0 if idle
  string last_output = 6;     // BR-MB-15: truncated to 500 chars server-side, never a raw dump
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
