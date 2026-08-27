# TASK-PRF-03-01: Add `dev_server_id`/`repo_path` to `CreateProjectRequest` and `tags` to `DevServer`

**From Solution:** SOL-PRF-03
**Priority:** P0 — every other task in this set depends on generated stubs from this
**Service:** `project-service` (+ `infra-fleet-service` proto)
**File:** `backend-go/proto/orca/project/v1/project.proto`
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

Dev-server binding at project creation and RBAC-filtered project listing
both need new wire fields before any usecase work can compile against them.
Both changes are additive-only — existing field numbers are untouched.

## Changes to make

In `backend-go/proto/orca/project/v1/project.proto`, extend
`CreateProjectRequest` (currently fields 1-5):

```protobuf
message CreateProjectRequest {
  string tenant_id = 1;
  string name = 2;
  string description = 3;
  string default_branch = 4;
  string visibility = 5;
  string dev_server_id = 6; // NEW — binds at creation time, per BL-PRF-03's flow
  string repo_path = 7;     // NEW — absolute path on dev_server_id; becomes the project's first Repo
}
```

In `backend-go/proto/orca/infrafleet/v1/infrafleet.proto`, extend `DevServer`
(currently fields 1-5) and its two request messages:

```protobuf
message DevServer {
  string id = 1;
  string tenant_id = 2;
  string host = 3;
  ConnectionMode mode = 4;
  string ssh_target_id = 5;
  repeated string tags = 6; // NEW — BL-PRF-03's allowedServerTags match target
}

message RegisterDevServerRequest {
  string tenant_id = 1;
  string host = 2;
  ConnectionMode mode = 3;
  string ssh_target_id = 4;
  repeated string tags = 5; // NEW
}
```

Check `UpdateDevServerRequest`'s current field list before editing (not
confirmed in this task's research) and add `repeated string tags` at the
next free field number, mirroring the same pattern.

## Verify

```bash
cd /opt/repos/orca/backend-go
buf generate proto
buf breaking proto --against '.git#branch=main'
go build ./proto/...
```

Expected: clean build; `buf breaking` reports only additions.
