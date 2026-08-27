# TASK-031: Add `BrowserProfile` message + `ListBrowserProfiles`/`CreateBrowserProfile`/`DeleteBrowserProfile` RPCs to `infrafleet.proto`

**From Solution:** SOL-006 (Group C — metadata CRUD half)
**Priority:** P1
**Service:** `infra-fleet-service`
**File:** `backend-go/proto/orca/infrafleet/v1/infrafleet.proto`
**Depends on:** none
**Status:** `[x]` DONE — implemented in worktree `agent-a0480f57a839cc758` (accounts/aiProvider/browser/credentials pass, merged into `integration/missing-v1` as commit `cc3eb16ba`); proto RPCs (ListBrowserProfiles/CreateBrowserProfile/DeleteBrowserProfile) confirmed present in infrafleet.proto and wired end-to-end — this task doc's own Status line was never updated by that pass (a task-doc-capture gap, not a missing-code gap).

---

## Context

SOL-006 Group C splits `browser.profileList`/`profileCreate`/
`profileDelete` off as Postgres-backed metadata CRUD in
`infra-fleet-service` (mirrors `ssh_targets`' shape), distinct from the 3
live-agent profile operations (`profileClearDefaultCookies`/
`profileDetectBrowsers`/`profileImportFromBrowser`, TASK-034). SOL-006's
own code sketch jumps straight to the usecase layer without spelling out
the gRPC surface those usecases need to be reachable from `api-gateway`
across the service boundary — this task adds that proto surface, filling
an implied-but-unwritten gap rather than inventing new design (the 3
Postgres CRUD operations and their request/response shapes are already
fully specified by SOL-006's table and SQL schema; only the RPC/message
wrapper is new).

---

## Changes to make

**File:** `backend-go/proto/orca/infrafleet/v1/infrafleet.proto`

### Step 1 — add 3 RPCs to the `InfraFleetService` service block

Add after the existing `Relay` RPC:

```protobuf
  // ListBrowserProfiles/CreateBrowserProfile/DeleteBrowserProfile back
  // api-gateway's browser.profileList/profileCreate/profileDelete channels
  // (SOL-006 Group C) — Postgres-backed metadata CRUD, mirroring
  // CreateSshTarget's shape. NOT the same as the 3 live-agent profile
  // operations (profileClearDefaultCookies/profileDetectBrowsers/
  // profileImportFromBrowser), which relay via Relay instead — see
  // specs/backend-go/bugs/missing-v1/solutions/SOL-006-browser-channels.md.
  rpc ListBrowserProfiles(ListBrowserProfilesRequest) returns (ListBrowserProfilesResponse);
  rpc CreateBrowserProfile(CreateBrowserProfileRequest) returns (CreateBrowserProfileResponse);
  rpc DeleteBrowserProfile(DeleteBrowserProfileRequest) returns (google.protobuf.Empty);
```

Add `import "google/protobuf/empty.proto";` at the top of the file if not
already present.

### Step 2 — append new messages to the bottom of the file

```protobuf
message BrowserProfile {
  string id = 1;
  string tenant_id = 2;
  string dev_server_id = 3;
  string name = 4;
  string source_browser = 5; // e.g. "chrome", "firefox" — set by profileImportFromBrowser; empty if manually created
  bool is_default = 6;
  google.protobuf.Timestamp created_at = 7;
}

message ListBrowserProfilesRequest {
  string dev_server_id = 1; // required — a profile is dev-server-scoped, not tenant-wide
}
message ListBrowserProfilesResponse {
  repeated BrowserProfile profiles = 1;
}

message CreateBrowserProfileRequest {
  string dev_server_id = 1;
  string name = 2;
  string source_browser = 3; // optional
  bool is_default = 4;
}
message CreateBrowserProfileResponse {
  BrowserProfile profile = 1;
}

message DeleteBrowserProfileRequest {
  string id = 1;
}
```

Add `import "google/protobuf/timestamp.proto";` at the top of the file if
not already present (needed for `BrowserProfile.created_at`).

---

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
