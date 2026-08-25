# TASK-162: Add `ListSshTargets`/`GetSshState`/`EstablishConnection` RPCs to `infrafleet.proto`

**From Solution:** SOL-024
**Priority:** P1
**Service:** `infra-fleet-service`
**File:** `backend-go/proto/orca/infrafleet/v1/infrafleet.proto`
**Depends on:** none
**Status:** `[x]` DONE (verified) — `ListSshTargets`/`GetSshState`/`EstablishConnection` RPCs + `SshTarget`/`Connection`/request/response messages added to `infrafleet.proto` (no name collisions — no prior `SshTarget`/`Connection` messages existed), stubs regenerated, `go build ./proto/...` clean. `buf breaking` not run (no usable git remote in this worktree) — additive-only, confirmed via `go build`.

---

## Context

`infra-fleet-service.md` §3's RPC sketch already names the target contract
under "SSH target registration" (`RegisterSshTarget`/`GetSshTarget`/
`ListSshTargets`/...) and "Connection lifecycle"
(`EstablishConnection`/`TeardownConnection`/...) — today's
`infrafleet.proto` only implements the create half of the first group
(`CreateSshTarget`) and none of the second. This task adds just what
`ssh.listTargets`/`ssh.getUserAccount`/`ssh.getState`/`ssh.connect` need:
`ListSshTargets` (backs both `ssh.listTargets` and `ssh.getUserAccount` —
see TASK-165), a local-read `GetSshState`, and a scoped-down
`EstablishConnection`. Does **not** add `UpdateSshTarget`/
`DeleteSshTarget`/`TeardownConnection`/`CheckConnectionHealth`/
`GetSshTarget` — none of the 4 missing channels need them
(`rpc-catalog.md:420-427`'s 4-method `ssh.*` list).

## Changes to make

**File:** `backend-go/proto/orca/infrafleet/v1/infrafleet.proto`

Add to the `InfraFleetService` service block:

```protobuf
  // ListSshTargets backs ssh.listTargets and ssh.getUserAccount (the
  // latter derives from this same read — see wscompat's registerSshChannels).
  rpc ListSshTargets(ListSshTargetsRequest) returns (ListSshTargetsResponse);
  // GetSshState is a local read (no dial) of whichever connection (if any)
  // currently binds this SSH target's dev server.
  rpc GetSshState(GetSshStateRequest) returns (GetSshStateResponse);
  // EstablishConnection performs the actual SSH + Dev Server Agent
  // handshake synchronously — it IS the connection-establishment act, not
  // a record of one requested. See usecase.EstablishConnection's doc comment.
  rpc EstablishConnection(EstablishConnectionRequest) returns (Connection);
```

Add the messages:

```protobuf
message SshTarget {
  string id = 1;
  string tenant_id = 2;
  string host = 3;
  string user = 4;
  string vault_ssh_role = 5; // a Vault role pointer, never key material — safe to return (infra-fleet-service.md §9)
}

message ListSshTargetsRequest {
  // tenant_id intentionally absent — pulled from context, same convention
  // as ListDevServersRequest.
}

message ListSshTargetsResponse {
  repeated SshTarget ssh_targets = 1;
}

message GetSshStateRequest {
  string ssh_target_id = 1;
}

message GetSshStateResponse {
  bool connected = 1;
  string connection_id = 2;       // empty when not connected
  int64 last_activity_unix_ms = 3; // 0 when not connected or never active
}

message EstablishConnectionRequest {
  string ssh_target_id = 1;
}

message Connection {
  string id = 1;                 // the connectionId every other RPC keys on
  string dev_server_id = 2;
  string status = 3;             // "established" | "degraded" | "closed"
  int64 established_at_unix_ms = 4;
}
```

If `SshTarget`/`Connection` message names collide with anything already
declared in this file (there is currently no `SshTarget` message —
`CreateSshTargetRequest`/`CreateSshTargetResponse` use inline fields, not
a shared message — verify before adding), rename to avoid a duplicate
declaration; keep both proto and Go usages consistent if you do.

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
