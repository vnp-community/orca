# TASK-127: Add `GetUserProfile`/`ListDepartments`/`UpdateCompany`/`UpdateDepartment`/`UpdateUserProfile` RPCs to `tenant.proto`

**From Solution:** SOL-019
**Priority:** P1 — TASK-128/TASK-129 depend on the generated stubs from this
**Service:** `tenant-service`
**File:** `backend-go/proto/orca/tenant/v1/tenant.proto`
**Depends on:** none
**Status:** `[x]` DONE — implemented in worktree `agent-a9271c5b2d89347e7`, **committed** as `19b216531`. Build/vet/test clean. Pending merge + one-line RegisterRealChannels/main.go wiring for `channels_tenant_project.go`.

---

## Context

`tenant-service.md` §3 already specifies these 5 RPCs; only a subset of the
service's designed surface actually shipped in `tenant.proto`. This task
closes that gap RPC-for-RPC. Additive only — no `buf breaking` risk.

`UpdateUserProfileRequest.department_id` needs an explicit way to
distinguish "no change" from "clear the department" (unlike every other
`Update*Request` in this codebase, `UserProfile`'s own semantics already
treat an empty `department_id` as meaningful — "company-only inheritance").
This task resolves that with a `clear_department` bool flag rather than
reusing the ambiguous empty-string idiom.

## Changes to make

**File:** `backend-go/proto/orca/tenant/v1/tenant.proto`

### Step 1: Add RPCs to the `TenantService` block

Find:

```protobuf
service TenantService {
  rpc CreateCompany(CreateCompanyRequest) returns (CreateCompanyResponse);
  rpc ValidateTenant(ValidateTenantRequest) returns (ValidateTenantResponse);
  rpc CreateDepartment(CreateDepartmentRequest) returns (CreateDepartmentResponse);
  rpc SetUserDepartment(SetUserDepartmentRequest) returns (SetUserDepartmentResponse);
  rpc GetResolvedProfile(GetResolvedProfileRequest) returns (GetResolvedProfileResponse);
  rpc CreateTeam(CreateTeamRequest) returns (CreateTeamResponse);
  rpc AddTeamMember(AddTeamMemberRequest) returns (AddTeamMemberResponse);
  rpc ListTeamMembers(ListTeamMembersRequest) returns (ListTeamMembersResponse);
}
```

Replace with:

```protobuf
service TenantService {
  rpc CreateCompany(CreateCompanyRequest) returns (CreateCompanyResponse);
  rpc ValidateTenant(ValidateTenantRequest) returns (ValidateTenantResponse);
  rpc CreateDepartment(CreateDepartmentRequest) returns (CreateDepartmentResponse);
  rpc SetUserDepartment(SetUserDepartmentRequest) returns (SetUserDepartmentResponse);
  rpc GetResolvedProfile(GetResolvedProfileRequest) returns (GetResolvedProfileResponse);
  rpc CreateTeam(CreateTeamRequest) returns (CreateTeamResponse);
  rpc AddTeamMember(AddTeamMemberRequest) returns (AddTeamMemberResponse);
  rpc ListTeamMembers(ListTeamMembersRequest) returns (ListTeamMembersResponse);

  // ── profile.* surface (tenant-service.md §3) ──────────────────────────
  rpc GetUserProfile(GetUserProfileRequest) returns (GetUserProfileResponse);
  rpc ListDepartments(ListDepartmentsRequest) returns (ListDepartmentsResponse);
  rpc UpdateCompany(UpdateCompanyRequest) returns (UpdateCompanyResponse);
  rpc UpdateDepartment(UpdateDepartmentRequest) returns (UpdateDepartmentResponse);
  rpc UpdateUserProfile(UpdateUserProfileRequest) returns (UpdateUserProfileResponse);
}
```

### Step 2: Append new messages to the bottom of the file

```protobuf
// UserProfile is the wire shape of tenant.user_profiles — mirrors
// domain.UserProfile.
message UserProfile {
  string user_id = 1;
  string company_id = 2;
  string department_id = 3;  // empty = company-only inheritance
  string settings_json = 4;
}

message GetUserProfileRequest {
  string user_id = 1;
}

message GetUserProfileResponse {
  UserProfile profile = 1;
}

message ListDepartmentsRequest {
  string company_id = 1;
}

message ListDepartmentsResponse {
  repeated Department departments = 1;
}

message UpdateCompanyRequest {
  string id = 1;
  string name = 2;          // empty = no change
  string settings_json = 3; // empty = no change
}

message UpdateCompanyResponse {
  Company company = 1;
}

message UpdateDepartmentRequest {
  string id = 1;
  string name = 2;          // empty = no change
  string settings_json = 3; // empty = no change
}

message UpdateDepartmentResponse {
  Department department = 1;
}

// UpdateUserProfileRequest: department_id is only applied when
// clear_department is true OR department_id is non-empty — this avoids the
// "" = no-change vs "" = clear ambiguity UserProfile's own semantics create
// (see this task's Context). settings_json empty = no change, same as
// every other Update*Request in this file.
message UpdateUserProfileRequest {
  string user_id = 1;
  string department_id = 2;  // ignored unless clear_department is also set
  bool clear_department = 3; // true = set department_id to empty ("company-only")
  string settings_json = 4;  // empty = no change
}

message UpdateUserProfileResponse {
  UserProfile profile = 1;
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
