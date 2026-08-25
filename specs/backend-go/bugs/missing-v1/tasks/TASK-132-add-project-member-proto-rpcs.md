# TASK-132: Add `ListMembers`/`RemoveMember`/`UpdateMemberRole` RPCs to `project.proto`

**From Solution:** SOL-020
**Priority:** P1 — TASK-133/TASK-134 depend on the generated stubs from this
**Service:** `project-service`
**File:** `backend-go/proto/orca/project/v1/project.proto`
**Depends on:** none
**Status:** `[x]` DONE (verified) — `ListMembers`/`RemoveMember`/`UpdateMemberRole` RPCs + `Member` message added to `project.proto`; `buf generate` regenerated stubs cleanly; `go build ./proto/...` green. `buf breaking` against `main` not meaningful (no `backend-go/` on `main` at all — confirmed via `git ls-tree`).

---

## Context

`project-service.md` §3 already specifies these 3 RPCs alongside the one
member RPC that shipped (`AddMember`). Additive only — no `buf breaking`
risk. Follows this proto's own established convention of an explicit empty
`*Response` wrapper (`AddMemberResponse{}`, `DeleteProjectResponse{}`)
rather than `google.protobuf.Empty`.

## Changes to make

**File:** `backend-go/proto/orca/project/v1/project.proto`

### Step 1: Add RPCs to the `ProjectService` block

Find:

```protobuf
  rpc AddMember(AddMemberRequest) returns (AddMemberResponse);
  rpc RebindDevServer(RebindDevServerRequest) returns (RebindDevServerResponse);
```

Replace with:

```protobuf
  rpc AddMember(AddMemberRequest) returns (AddMemberResponse);
  rpc ListMembers(ListMembersRequest) returns (ListMembersResponse);
  rpc RemoveMember(RemoveMemberRequest) returns (RemoveMemberResponse);
  rpc UpdateMemberRole(UpdateMemberRoleRequest) returns (UpdateMemberRoleResponse);
  rpc RebindDevServer(RebindDevServerRequest) returns (RebindDevServerResponse);
```

### Step 2: Append new messages, right after `AddMemberResponse {}`

Find:

```protobuf
message AddMemberResponse {}
```

Replace with:

```protobuf
message AddMemberResponse {}

message Member {
  string user_id = 1;
  ProjectRole role = 2;
}

message ListMembersRequest {
  string project_id = 1;
}

message ListMembersResponse {
  repeated Member members = 1;
}

message RemoveMemberRequest {
  string project_id = 1;
  string user_id = 2;
}

message RemoveMemberResponse {}

message UpdateMemberRoleRequest {
  string project_id = 1;
  string user_id = 2;
  ProjectRole role = 3;
}

message UpdateMemberRoleResponse {
  Member member = 1;
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
