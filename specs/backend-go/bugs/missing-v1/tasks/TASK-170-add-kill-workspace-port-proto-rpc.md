# TASK-170: Add `KillWorkspacePort` RPC to `infrafleet.proto`

**From Solution:** SOL-027
**Priority:** P1
**Service:** `infra-fleet-service`
**File:** `backend-go/proto/orca/infrafleet/v1/infrafleet.proto`
**Depends on:** none
**Status:** `[x]` DONE (verified) — `KillWorkspacePort` RPC + `KillWorkspacePortRequest`/`KillWorkspacePortResponse` added to `infrafleet.proto`, stubs regenerated, `go build ./proto/...` clean. `buf breaking` not run (no usable git remote in this worktree) — additive-only, confirmed via `go build`.

---

## Context

`ScanWorkspacePorts` exists; `KillWorkspacePort` doesn't exist at any
layer. This task does not invent a different transport for `kill` — it
follows the exact `resolve → (relay to agent | local no-op)` shape
`ScanWorkspacePorts` already established, per the same instruction not to
reproduce the old TS bug class (`workspacePorts.scan`/`kill` silently
dropping remote-connected repos,
`specs/frontend/api/backend-agent-execution-boundary.md:118,179`).

## Changes to make

**File:** `backend-go/proto/orca/infrafleet/v1/infrafleet.proto`

Add to the `InfraFleetService` service block, next to
`ScanWorkspacePorts`:

```protobuf
  rpc KillWorkspacePort(KillWorkspacePortRequest) returns (KillWorkspacePortResponse);
```

Add the messages, next to `ScanWorkspacePortsRequest`/
`ScanWorkspacePortsResponse`:

```protobuf
message KillWorkspacePortRequest {
  string connection_id = 1;
  string worktree_id = 2;
  int32 pid = 3;
  int32 port = 4;
}

message KillWorkspacePortResponse {
  bool ok = 1;
  string reason = 2; // populated only when ok is false
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
