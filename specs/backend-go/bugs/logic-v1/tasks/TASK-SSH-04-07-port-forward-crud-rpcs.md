# TASK-SSH-04-07: `CreatePortForward`/`ListPortForwards`/`DeletePortForward` RPCs + fix `ScanWorkspacePortsResponse` shape

**From Solution:** SOL-SSH-04
**Priority:** P1
**Service:** `infra-fleet-service`
**File:** `backend-go/proto/orca/infrafleet/v1/infrafleet.proto`
**Depends on:** TASK-SSH-04-01, TASK-SSH-04-03
**Status:** `[ ]` TODO

---

## Context

`infra-fleet-service.md` §3 already sketches `CreatePortForward`/
`ListPortForwards`/`DeletePortForward` — completing already-designed RPCs,
not inventing them. This also finishes TASK-SSH-04-01's usecase-layer
return-type change by updating the gRPC boundary: `ScanWorkspacePortsResponse`
today is `repeated int32 open_ports = 1`, which can never carry the
`{port, process}` pair BUG-SSH-04/BL-SSH-04 require.

## Changes to make

`backend-go/proto/orca/infrafleet/v1/infrafleet.proto`:

```protobuf
  rpc CreatePortForward(CreatePortForwardRequest) returns (PortForward);
  rpc ListPortForwards(ListPortForwardsRequest) returns (ListPortForwardsResponse);
  rpc DeletePortForward(DeletePortForwardRequest) returns (google.protobuf.Empty);
```

```protobuf
message PortForward {
  string id = 1;
  string connection_id = 2;
  int32 local_port = 3;
  int32 remote_port = 4;
  string process_name = 5;
  string status = 6; // "active" | "closed"
}
message CreatePortForwardRequest {
  string connection_id = 1;
  int32 remote_port = 2;
}
message ListPortForwardsRequest {
  string connection_id = 1;
}
message ListPortForwardsResponse {
  repeated PortForward port_forwards = 1;
}
message DeletePortForwardRequest {
  string id = 1;
}
```

Replace `ScanWorkspacePortsResponse` (currently `repeated int32 open_ports = 1`):

```protobuf
message DetectedPortProto {
  int32 port = 1;
  string host = 2;
  int32 pid = 3;
  string process_name = 4;
}

message ScanWorkspacePortsResponse {
  repeated DetectedPortProto ports = 1;
}
```

This is a breaking proto change (field removed, not just added) — flag it
explicitly in the PR description per this repo's `buf breaking` convention;
`open_ports` had no real consumer yet per `infra-fleet-service.md` §10's
"schema only" framing for this whole area, so the break has no live caller
to migrate.

Regenerate: `buf generate proto` from `backend-go/`.

Update the gRPC server (`internal/adapter/grpc/server.go`):
`ScanWorkspacePorts`'s response mapping now builds
`[]*infrafleetv1.DetectedPortProto` from `usecase.DetectedPort`; add
`CreatePortForward`/`ListPortForwards`/`DeletePortForward` handlers
delegating to new `usecase.CreatePortForward`/`ListPortForwards`/
`DeletePortForward` types (straightforward CRUD over
`PortForwardRepository` from TASK-SSH-04-03 — mirror `CreateSshTarget`'s
shape).

## Verify

```bash
cd /opt/repos/orca/backend-go
buf generate proto
buf breaking proto --against '.git#branch=main' || echo "expected: reports the open_ports removal — confirm no other consumer exists before merging"
go build ./...
go test ./services/infra-fleet-service/... -v
```

Expected: build clean; `ScanWorkspacePorts` gRPC handler returns
`{port, host, pid, processName}` entries; `CreatePortForward`/
`ListPortForwards`/`DeletePortForward` round-trip through
`PortForwardStore`.
