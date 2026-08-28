# TASK-FLEET-01-01: Add `project`/`tags` to `SshTarget` and `ImportFleetInventory` RPC to `infrafleet.proto`

**From Solution:** SOL-FLEET-01
**Priority:** P0 — everything else in this solution depends on generated stubs from this
**Service:** `infra-fleet-service` (proto)
**File:** `backend-go/proto/orca/infrafleet/v1/infrafleet.proto`
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

BL-FLEET-01's YAML import flow needs `project`/`tags` on `SshTarget` and a
batch `ImportFleetInventory` RPC that neither exists in `infrafleet.proto`
today. Additive-only change, so `buf breaking` stays clean.

## Changes to make

In the existing `SshTarget` message, append two fields (keep existing field
numbers untouched, use the next free numbers):

```protobuf
message SshTarget {
  // ... existing fields unchanged ...
  string project = 6;  // new — "" = ungrouped
  repeated string tags = 7;  // new
}
```

Add new messages (append to the bottom of the file):

```protobuf
message FleetServerInput {
  string host = 1;
  string user = 2;
  string vault_ssh_role = 3;
  string project = 4;
  repeated string tags = 5;
}

message ImportFleetInventoryRequest {
  repeated FleetServerInput servers = 1;
  bool dry_run = 2;
}
message ImportFleetInventoryError { string host = 1; string user = 2; string reason = 3; }
message ImportFleetInventoryResponse {
  int32 imported = 1;
  int32 updated = 2;
  int32 skipped = 3;
  repeated ImportFleetInventoryError errors = 4;
}
```

Add to the `InfraFleetService` service block:

```protobuf
rpc ImportFleetInventory(ImportFleetInventoryRequest) returns (ImportFleetInventoryResponse);
```

Note: check the actual next-free field numbers on `SshTarget` in the current
file before applying — the numbers above (6, 7) assume the message currently
ends at field 5; adjust if that has drifted.

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

Expected: clean build, `buf breaking` reports no breaking changes (only additions).
