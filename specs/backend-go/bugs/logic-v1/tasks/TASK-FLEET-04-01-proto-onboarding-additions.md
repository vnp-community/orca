# TASK-FLEET-04-01: Proto — `DetectDevServerAgents`/`CheckDevServerPreflight` RPCs, `RegisterDevServerRequest.relay_port`

**From Solution:** SOL-FLEET-04
**Priority:** P0
**Service:** `infra-fleet-service` (proto)
**File:** `backend-go/proto/orca/infrafleet/v1/infrafleet.proto`
**Depends on:** TASK-FLEET-02-06 (adds `DevServer.status`; this task adds the sibling platform fields to the same message — coordinate field numbers)
**Status:** [x] DONE — DevServer.status already claimed field 6 (not 8/9 as drafted); platform/arch/node_version/agent_version use the actual next-free 7-10. RegisterDevServerRequest.relay_port=5 matched exactly. Added DetectDevServerAgents/CheckDevServerPreflight RPCs + messages. Also updated toProtoDevServer to populate the new fields (was previously silently dropping them). `go build ./proto/...` and full service build/test clean.

---

## Context

Closes BL-FLEET-04 Steps 3/4 (agent detection, remote preflight) and the
Step 6 field gap (`relay_port`). `DevServer.Platform/Arch/NodeVersion/
AgentVersion` are the same columns SOL-FLEET-02 introduces
(`domain.DevServer`) — this task is the proto-message side of that shared
field set, so it must be sequenced after (or coordinated with)
TASK-FLEET-02-06 to avoid a field-number collision on `DevServer`.

## Changes to make

Add to the existing `DevServer` message (use the next free field numbers
after whatever SOL-FLEET-02's `status` field claimed):

```protobuf
string platform = 9;
string arch = 10;
string node_version = 11;
string agent_version = 12;
```

Add to `RegisterDevServerRequest`:

```protobuf
int32 relay_port = 5; // 0 = no daemon port — foreground stdio session, honest placeholder until agent/ gains a daemon (see TASK-FLEET-02-08)
```

Add new messages + RPCs:

```protobuf
message AgentProbe {
  string id = 1;
  string cmd = 2;
  repeated string required_commands = 3;
  repeated string unsupported_runtimes = 4;
}
message DetectDevServerAgentsRequest {
  string dev_server_id = 1;
  repeated AgentProbe commands = 2;
}
message DetectDevServerAgentsResponse {
  repeated string agents = 1;
  string platform = 2;
}

message CheckDevServerPreflightRequest {
  string dev_server_id = 1;
  int32 probe_port = 2;
}
message CheckResult { bool installed = 1; string version = 2; bool meets_min = 3; }
message DiskCheckResult { double free_gb = 1; bool meets_min = 2; }
message PortCheckResult { int32 port = 1; bool available = 2; }
message CheckDevServerPreflightResponse {
  CheckResult git = 1;
  CheckResult node = 2;
  DiskCheckResult disk = 3;
  PortCheckResult port = 4;
  CheckResult gh = 5;
}

service InfraFleetService {
  // ...
  rpc DetectDevServerAgents(DetectDevServerAgentsRequest) returns (DetectDevServerAgentsResponse);
  rpc CheckDevServerPreflight(CheckDevServerPreflightRequest) returns (CheckDevServerPreflightResponse);
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

Expected: clean build, `buf breaking` reports no breaking changes.
