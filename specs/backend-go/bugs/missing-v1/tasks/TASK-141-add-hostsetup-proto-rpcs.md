# TASK-141: Add `HostSetup` message + 5 `projectHostSetup.*` RPCs to `project.proto`

**From Solution:** SOL-022
**Priority:** P1 — TASK-143/TASK-144 depend on the generated stubs from this
**Service:** `project-service`
**File:** `backend-go/proto/orca/project/v1/project.proto`
**Depends on:** none
**Status:** `[x]` DONE — implemented in worktree `agent-a9271c5b2d89347e7`, **committed** as `19b216531`. Build/vet/test clean. Pending merge + one-line RegisterRealChannels/main.go wiring for `channels_tenant_project.go`.

---

## Context

**Ownership: `project-service`, not `infra-fleet-service`** —
`infra-fleet-service` owns reachability (dev servers, connections) and has
no concept of a *project*; `project-service` already owns the adjacent,
structurally similar concepts (`Project.dev_server_id`, `Repo`,
`RebindDevServer`'s host-validation saga). `dev_server_id` here is a
**logical FK, ID-only** — the same convention `project-service.md` §5
already uses for `projects.dev_server_id` itself
(`05-data-architecture.md`'s "no cross-database FK" rule).

`projectHostSetup` models the pre-project wizard step: name a dev server +
an existing absolute folder path on it, validate that path on the dev
server (never `project-service`'s own host), and on `setupExistingFolder`
finalize it into a real `Project` + `Repo`.

**Simplification vs. SOL-022's original sketch**: SOL-022 proposed also
adding a `path` field to `AddRepoRequest` (a remote-clone-URL-shaped
message with no room for "this is already a folder, not something to
clone"). This task does **not** add that field — `SetupExistingFolder`
(TASK-143) instead reuses the existing `url` field to carry the absolute
on-disk path, the same reuse `TASK-138`'s `ImportNested` insert already
does for the same reason (see that task's Context for the flagged
follow-up: give `project.repos` a real `path` column later, migrate both
call sites off `url` together). This keeps this task additive-only against
a message another SOL (SOL-021, TASK-137) may have already added RPCs to.

Additive only — no `buf breaking` risk.

## Changes to make

**File:** `backend-go/proto/orca/project/v1/project.proto`

### Step 1: Add RPCs to the `ProjectService` block

Add at the end of the `service ProjectService { ... }` block, before its
closing `}` (after whatever SOL-021/TASK-137 added, if that task has
already landed — otherwise right after the `ProjectGroup` surface's `rpc
ListProjectGroups(...)`):

```protobuf
  // ── projectHostSetup.* — pre-project dev-server-folder wizard ─────────
  rpc CreateHostSetup(CreateHostSetupRequest) returns (CreateHostSetupResponse);
  rpc ListHostSetups(ListHostSetupsRequest) returns (ListHostSetupsResponse);
  rpc UpdateHostSetup(UpdateHostSetupRequest) returns (UpdateHostSetupResponse);
  rpc DeleteHostSetup(DeleteHostSetupRequest) returns (DeleteHostSetupResponse);
  rpc SetupExistingFolder(SetupExistingFolderRequest) returns (SetupExistingFolderResponse);
```

### Step 2: Append new messages to the bottom of the file

```protobuf
message HostSetup {
  string id = 1;
  string tenant_id = 2;
  string dev_server_id = 3; // logical FK -> infra-fleet-service.dev_servers
  string folder_path = 4;
  string display_name = 5;
  string status = 6;        // pending | validated | completed | failed
  string project_id = 7;    // empty until completed
}

message CreateHostSetupRequest {
  string dev_server_id = 1;
  string folder_path = 2;
  string display_name = 3;
}
message CreateHostSetupResponse {
  HostSetup setup = 1;
}

message ListHostSetupsRequest {}
message ListHostSetupsResponse {
  repeated HostSetup setups = 1;
}

message UpdateHostSetupRequest {
  string id = 1;
  string folder_path = 2;  // empty = no change
  string display_name = 3; // empty = no change
}
message UpdateHostSetupResponse {
  HostSetup setup = 1;
}

message DeleteHostSetupRequest {
  string id = 1;
}
message DeleteHostSetupResponse {}

// SetupExistingFolder validates folder_path exists on dev_server_id
// (relayed to the Dev Server Agent via infra-fleet-service, never checked
// against project-service's own host), then creates a real Project + Repo
// from it and marks the HostSetup completed (or failed).
message SetupExistingFolderRequest {
  string id = 1; // the HostSetup being finalized
}
message SetupExistingFolderResponse {
  HostSetup setup = 1;  // status now "completed" or "failed"
  Project project = 2;  // set only on success
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
