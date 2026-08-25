# TASK-137: Add `ProjectGroup.project_id` field + `MoveProject`/`ScanNested`/`ImportNested` RPCs to `project.proto`

**From Solution:** SOL-021
**Priority:** P1 — TASK-138/TASK-139 depend on the generated stubs from this
**Service:** `project-service`
**File:** `backend-go/proto/orca/project/v1/project.proto`
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

Unlike SOL-019/SOL-020, `project-service.md` does **not** pre-specify these
3 RPCs — they are genuinely new. But the data model they need already
exists, partially: `project-service.md` §5's `project_groups` table has a
nullable `project_id` column ("a group can pre-date any linked project
during nested-repo import"). The shipped `ProjectGroup` message doesn't have
this field yet — this task adds it.

`scanNested`/`importNested` relay through the **dev server** (via
infra-fleet-service's `Relay`/`CreateConnection`), never
`project-service`'s own host — see TASK-138 for the design rationale
(closes the same "legacy desktop-app assumption" bug class BUG-021 flags).

**Open dependency, called out explicitly**: the JSON-RPC method name
`fs.scanNestedRepos` this solution relays is a proposal, not confirmed to
exist on the Dev Server Agent today. Flag this to the team before relying on
`ScanNested` actually returning real candidates end-to-end — `Relay` will
forward the call either way, but the Agent-side handler is out of scope for
`backend-go` and needs its own confirmation/implementation.

Additive only — no `buf breaking` risk.

## Changes to make

**File:** `backend-go/proto/orca/project/v1/project.proto`

### Step 1: Add `project_id` to `ProjectGroup`

Find:

```protobuf
message ProjectGroup {
  string id = 1;
  string tenant_id = 2;
  string name = 3;
  string parent_group_id = 4; // empty = root of its tree
}
```

Replace with:

```protobuf
message ProjectGroup {
  string id = 1;
  string tenant_id = 2;
  string name = 3;
  string parent_group_id = 4; // empty = root of its tree
  string project_id = 5;      // empty = pure folder node, not yet linked to a project
}
```

### Step 2: Add RPCs to the `ProjectService` block

Find:

```protobuf
  rpc CreateProjectGroup(CreateProjectGroupRequest) returns (CreateProjectGroupResponse);
  rpc UpdateProjectGroup(UpdateProjectGroupRequest) returns (UpdateProjectGroupResponse);
  rpc DeleteProjectGroup(DeleteProjectGroupRequest) returns (DeleteProjectGroupResponse);
  rpc ListProjectGroups(ListProjectGroupsRequest) returns (ListProjectGroupsResponse);
}
```

Replace with:

```protobuf
  rpc CreateProjectGroup(CreateProjectGroupRequest) returns (CreateProjectGroupResponse);
  rpc UpdateProjectGroup(UpdateProjectGroupRequest) returns (UpdateProjectGroupResponse);
  rpc DeleteProjectGroup(DeleteProjectGroupRequest) returns (DeleteProjectGroupResponse);
  rpc ListProjectGroups(ListProjectGroupsRequest) returns (ListProjectGroupsResponse);

  // MoveProject/ScanNested/ImportNested: nested-repo import workflow —
  // project-service.md §5's project_groups.project_id column, first
  // exercised by these RPCs. See TASK-138's usecase doc comments for the
  // dev-server-relay design (never project-service's own filesystem).
  rpc MoveProject(MoveProjectRequest) returns (MoveProjectResponse);
  rpc ScanNested(ScanNestedRequest) returns (ScanNestedResponse);
  rpc ImportNested(ImportNestedRequest) returns (ImportNestedResponse);
}
```

### Step 3: Append new messages, right after `ListProjectGroupsResponse`

```protobuf
message MoveProjectRequest {
  string project_id = 1;
  string target_parent_group_id = 2; // empty = move to tree root
}

message MoveProjectResponse {
  ProjectGroup group = 1; // the project's (possibly newly-created) leaf group node, relocated
}

message ScanNestedRequest {
  string dev_server_id = 1;
  string root_path = 2; // absolute path on that dev server to scan under
}

message NestedRepoCandidate {
  string path = 1;
  string suggested_name = 2;
  bool is_git_repo = 3;
}

message ScanNestedResponse {
  repeated NestedRepoCandidate candidates = 1;
}

message ImportNestedRequest {
  string dev_server_id = 1;
  string parent_group_id = 2; // where the imported tree attaches; empty = root
  repeated NestedRepoCandidate selected = 3; // subset of a prior ScanNested's candidates the caller chose
}

message ImportNestedResponse {
  repeated ProjectGroup created_groups = 1;
  repeated Project created_projects = 2;
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
