# TASK-TG-03-04: Proto — `Grant` id/expiry, `action` on `ResolvePermissionRequest`, `RevokeGrant`/`ListGrants` RPCs

**From Solution:** SOL-TG-03
**Priority:** P1
**Service:** `task-service`
**File:** `backend-go/proto/orca/task/v1/task.proto`
**Depends on:** TASK-TG-01-01 (both touch `task.proto` — land after to avoid conflicting field-number allocation)
**Status:** `[ ]` TODO

---

## Context

Three wire-contract gaps `README.md:190-236` already names: no
`expires_at` on a grant, no `RevokeGrant`/`ListGrants` RPCs, and
`ResolvePermissionRequest` has no `action` field (the gRPC handler
hardcodes `Action: "read"` today). This task closes all three at the proto
layer only.

## Changes to make

In `backend-go/proto/orca/task/v1/task.proto`, add to the `TaskService`
service block:

```protobuf
  rpc RevokeGrant(RevokeGrantRequest) returns (google.protobuf.Empty);
  rpc ListGrants(ListGrantsRequest) returns (ListGrantsResponse);
```

Widen `GrantRequest`/`GrantResponse` and `ResolvePermissionRequest`:

```protobuf
message GrantRequest {
  string task_id = 1;
  string subject_id = 2; // user or team id
  GrantLevel level = 3;
  bool apply_tree = 4;
  google.protobuf.Timestamp expires_at = 5; // optional; unset = never expires
}

message GrantResponse {
  string id = 1; // new — the persisted grant's id, needed by RevokeGrant callers
}

message ResolvePermissionRequest {
  string task_id = 1;
  string user_id = 2;
  string action = 3; // new — closes README.md's "no action field on the wire" gap
}
```

Append new messages:

```protobuf
message RevokeGrantRequest {
  string task_id = 1;
  string grant_id = 2;
}

message ListGrantsRequest {
  string task_id = 1;
}
message Grant {
  string id = 1;
  string task_id = 2;
  string subject_id = 3;
  GrantLevel level = 4;
  bool apply_tree = 5;
  google.protobuf.Timestamp expires_at = 6;
}
message ListGrantsResponse {
  repeated Grant grants = 1;
}
```

Add `import "google/protobuf/timestamp.proto";` if not already added by
`TASK-TG-01-01` (both tasks touch this file — check before adding a
duplicate import line).

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

Expected: clean build. `buf breaking` flags: `GrantResponse` gaining field
1 is additive (was `{}` — empty messages have no existing field numbers to
collide with); `ResolvePermissionRequest.action` at field 3 is additive.
Confirm no other in-flight task-service task claimed proto field number 3
on `ResolvePermissionRequest` before this lands.
