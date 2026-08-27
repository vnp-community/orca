# TASK-176: Add `ListTeams`/`RemoveTeamMember` RPCs to `tenant.proto`

**From Solution:** SOL-028
**Priority:** P0 — everything else in this solution depends on generated stubs from this
**Service:** `tenant-service`
**File:** `backend-go/proto/orca/tenant/v1/tenant.proto`
**Depends on:** none
**Status:** `[x]` DONE — implemented in worktree `agent-aa8bd8599a599323a` (team/terminal/workflow/worktree pass, merged into `integration/missing-v1` as commit `baa34819a`); this task doc's own Status line was never updated by that implementing pass (a task-doc-capture gap, not a missing-code gap) — verified against the current merged code+tests during a later re-audit: build/vet/test clean.

---

## Context

`tenant.proto`'s current `TenantService` only has `CreateTeam`/`AddTeamMember`/
`ListTeamMembers` — `tenant-service.md` §3 already specifies `ListTeams` and
`RemoveTeamMember` too (`tenant-service.md:82,84`), and
`services/tenant-service/README.md:101` lists `RemoveTeamMember` explicitly
under "Known gaps / follow-ups." This task closes the proto gap only —
additive, so `buf breaking` stays clean.

## Changes to make

**File:** `backend-go/proto/orca/tenant/v1/tenant.proto`

Add two RPCs to the `TenantService` block, after `ListTeamMembers`:

```protobuf
  rpc ListTeamMembers(ListTeamMembersRequest) returns (ListTeamMembersResponse);

  // ListTeams(ListTeamsRequest) returns (ListTeamsResponse) — the missing
  // half of team.* CRUD (create/get exist; list never did). See
  // tenant-service.md §3.
  rpc ListTeams(ListTeamsRequest) returns (ListTeamsResponse);

  // RemoveTeamMember — documented gap, services/tenant-service/README.md:101.
  rpc RemoveTeamMember(RemoveTeamMemberRequest) returns (google.protobuf.Empty);
}
```

(Adjust the closing brace: `ListTeamMembers` is currently the last RPC in
the service block, so both new RPCs go directly above the existing closing
`}`.)

Since this proto file does not yet import `google/protobuf/empty.proto`,
add the import at the top of the file, alongside the existing `syntax`/
`package`/`option go_package` lines:

```protobuf
import "google/protobuf/empty.proto";
```

Add the new messages at the bottom of the file:

```protobuf
message ListTeamsRequest {
  // company_id intentionally omitted — same pattern AddTeamMemberRequest/
  // SetUserDepartmentRequest already use: the scoping company comes from
  // the validated request context (tenant.RequireTenantID), never a
  // client-supplied field, per tenant-service.md §9's "never inferred from
  // a nested resource ID" rule.
}

message ListTeamsResponse {
  repeated Team teams = 1;
}

message RemoveTeamMemberRequest {
  string team_id = 1;
  string user_id = 2;
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
additions — two new RPCs, two new messages, one new import).
