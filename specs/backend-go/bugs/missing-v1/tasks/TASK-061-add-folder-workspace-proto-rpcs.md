# TASK-061: Add `FolderWorkspace` RPCs to `project.proto`

**From Solution:** SOL-010 (Design — Proto additions)
**Priority:** P0 — every other `folderWorkspace.*` task depends on generated stubs from this
**Service:** `project-service`
**File:** `backend-go/proto/orca/project/v1/project.proto`
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

BUG-010 found `folderWorkspace.*` (standalone, non-git filesystem paths
added directly to the workspace) has no backend-go implementation.
SOL-010 adds a new, standalone `FolderWorkspace` entity to
`project-service`'s existing proto package — **not** a repurposing of
`ProjectGroup` (a distinct grouping concept with no `path` field) and
**not** a new service, since `project-service` already owns
`/v1/projects` and the closest structural sibling (`Repo`).

`GetFolderWorkspacePathStatus` is a **DB-conflict check, not a live
filesystem probe** — per BUG-010's dispatch-model finding,
`folderWorkspace.*` is Postgres-only in the old backend with no relay to
the Dev Server Agent. `PATH_STATUS_AVAILABLE` means "nothing in
`project-service` already claims this path," not "this directory exists
on disk." This is a deliberate, documented divergence — do not add a real
fs-existence check as part of this task.

---

## Changes to make

**File:** `backend-go/proto/orca/project/v1/project.proto`

### Step 1: Add RPCs to `ProjectService`

```protobuf
service ProjectService {
  // ... existing RPCs unchanged ...

  // ── Folder workspaces — standalone, non-git filesystem paths added
  // directly to the workspace. No project_id — see FolderWorkspace's
  // doc comment below. ──────────────────────────────────────────────
  rpc CreateFolderWorkspace(CreateFolderWorkspaceRequest) returns (FolderWorkspace);
  rpc UpdateFolderWorkspace(UpdateFolderWorkspaceRequest) returns (FolderWorkspace);
  rpc DeleteFolderWorkspace(DeleteFolderWorkspaceRequest) returns (google.protobuf.Empty);
  rpc ListFolderWorkspaces(ListFolderWorkspacesRequest) returns (ListFolderWorkspacesResponse);
  rpc GetFolderWorkspacePathStatus(GetFolderWorkspacePathStatusRequest) returns (GetFolderWorkspacePathStatusResponse);
}
```

### Step 2: Add messages (append to the bottom of the file)

```protobuf
// FolderWorkspace is a standalone, non-git filesystem path added directly
// to the workspace — distinct from ProjectGroup (a project-organizing
// tree node with no path field) and from Repo (git-backed, owned by a
// Project). See specs/backend-go/bugs/missing-v1/solutions/SOL-010-folderworkspace-channels.md.
message FolderWorkspace {
  string id = 1;
  string dev_server_id = 2;   // logical FK -> infra-fleet-service.dev_servers
  string path = 3;            // absolute path on the bound dev server
  string name = 4;            // display name (defaults to basename(path) if unset on create)
  string added_by = 5;        // logical FK -> tenant-service.users
  google.protobuf.Timestamp created_at = 6;
}

message CreateFolderWorkspaceRequest {
  string dev_server_id = 1;
  string path = 2;
  string name = 3;
}

message UpdateFolderWorkspaceRequest {
  string id = 1;
  string name = 2;   // the only mutable field — path/dev_server_id are re-add, not edit
}

message DeleteFolderWorkspaceRequest { string id = 1; }
message ListFolderWorkspacesRequest {}   // tenant-scoped, no filter params observed at the frontend call site
message ListFolderWorkspacesResponse { repeated FolderWorkspace folder_workspaces = 1; }

// GetFolderWorkspacePathStatus answers purely from project-service's own
// tables — a DB-conflict check, NOT a live filesystem probe. See this
// RPC's design note in SOL-010 before changing that assumption.
message GetFolderWorkspacePathStatusRequest {
  string dev_server_id = 1;
  string path = 2;
}
message GetFolderWorkspacePathStatusResponse {
  // PATH_STATUS_AVAILABLE | PATH_STATUS_ALREADY_FOLDER_WORKSPACE |
  // PATH_STATUS_ALREADY_REPO | PATH_STATUS_INVALID
  string status = 1;
  string existing_folder_workspace_id = 2;  // set when status == ALREADY_FOLDER_WORKSPACE
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

Expected: clean build, `buf breaking` reports no breaking changes (only additions).
