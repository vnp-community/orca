# TASK-102: Extend `issuetracking.proto` with Linear-only RPCs (`Team`/`Label`/`Member`/`CustomView`, `CreateProject`/`GetProject`)

**From Solution:** SOL-016
**Priority:** P0 — everything else in SOL-016 depends on generated stubs from this
**Service:** `issue-tracking-service`
**File:** `backend-go/proto/orca/issuetracking/v1/issuetracking.proto`
**Depends on:** TASK-096
**Status:** `[x]` DONE (verified) — CreateProject/GetProject/ListTeams/ListTeamLabels/ListTeamMembers/GetCustomView/ListWorkflowStates RPCs + messages + CreateIssueRequest.team_id/state_id fields added. `buf breaking` clean, `go build ./proto/...` clean.

---

## Context

SOL-016 reuses SOL-015's connection/credential/issue-CRUD/metadata RPCs
unchanged for `linear.*` (`GetConnectionStatus`, `Connect`, `SearchIssues`,
`ListIssues`, `CreateIssue`, `GetIssue`, `UpdateIssue`, `AddIssueComment`,
`ListIssueComments`, `ListAssignableUsers` — see SOL-016's "shared vs.
diverges" table). Four `linear.*` methods have no Jira analog at all
(`listTeams`, `teamLabels`, `teamMembers`, `getCustomView`) — SOL-016
explicitly rejects folding `Team` into `Project` (see SOL-016's own
rationale: a `Team` carries workflow-state/label/membership ownership a
Jira `Project` doesn't have). `createProject`/`getProject` ARE shared
concepts (both providers have "a bounded set of issues with its own
name/lead/status") and get real RPCs here too, per the TDD's own §3 listing.

This task only appends to the service block and message set TASK-096
already created — it does not touch any TASK-096 RPC/message.

## Changes to make

**File:** `backend-go/proto/orca/issuetracking/v1/issuetracking.proto`

### 1. Add to the `IssueTrackingService` service block

Find the comment `// ── SOL-016 (linear.*) appends its own RPCs here — see
TASK-102 ──` (added by TASK-096) and replace it with:

```protobuf
  // ── Linear project/team surface (new, SOL-016) ──────────────────────
  rpc CreateProject(CreateProjectRequest) returns (Project); // already TDD-sketched, §3
  rpc GetProject(GetProjectRequest) returns (Project);        // already TDD-sketched, §3

  // Linear-only — no forced Jira mapping, see SOL-016's "genuinely
  // diverges" table for why Team is not folded into Project.
  rpc ListTeams(ListTeamsRequest) returns (ListTeamsResponse);
  rpc ListTeamLabels(ListTeamLabelsRequest) returns (ListTeamLabelsResponse);
  rpc ListTeamMembers(ListTeamMembersRequest) returns (ListTeamMembersResponse);
  rpc GetCustomView(GetCustomViewRequest) returns (CustomView);

  // ListWorkflowStates backs linear.teamStates — SOL-016's own mapping
  // table maps teamStates onto "ListWorkflowStates ... Linear's per-team
  // ordered state list is exactly the WorkflowState concept §4 already
  // generalizes" but no earlier task actually added this RPC; added here
  // rather than silently left a dangling reference. team_id scopes it to
  // one Linear team; Jira has no per-project ordered-workflow-state
  // listing RPC today (GetProjectStatusOrder is the closest Jira analog,
  // TASK-096, but it groups by Kanban column, not a flat ordered list), so
  // this stays Linear-only for now, same as ListTeams/ListTeamLabels/
  // ListTeamMembers/GetCustomView above.
  rpc ListWorkflowStates(ListWorkflowStatesRequest) returns (ListWorkflowStatesResponse);
```

### 2. Add messages (append to the bottom of the file)

```protobuf
// Team is deliberately its own message, not a repurposed Project — see
// SOL-016's "genuinely diverges" table: a Team owns workflow states
// (teamStates), labels (teamLabels), and membership (teamMembers), none of
// which a Jira Project has in this proto.
message Team {
  string id = 1;
  string workspace_id = 2;
  string name = 3;
  string key = 4; // Linear's short team key, e.g. "ENG"
}

message Label { string id = 1; string name = 2; string color = 3; }

message Member { string id = 1; string display_name = 2; string avatar_url = 3; }

// CustomView has no Jira analog — Linear-only concept, not force-fit into
// any shared message (see SOL-016's rationale table).
message CustomView {
  string id = 1;
  string workspace_id = 2;
  string name = 3;
  string model = 4; // "issue" | "project"
  string team_id = 5; // optional — a view can be workspace-scoped or team-scoped
}

message CreateProjectRequest {
  string workspace_id = 1;
  string team_id = 2; // Linear projects are created within a team
  string name = 3;
  string description = 4;
}

message GetProjectRequest {
  string project_id = 1;
  string workspace_id = 2;
}

message ListTeamsRequest { string workspace_id = 1; }
message ListTeamsResponse { repeated Team teams = 1; }

message ListTeamLabelsRequest { string team_id = 1; string workspace_id = 2; }
message ListTeamLabelsResponse { repeated Label labels = 1; }

message ListTeamMembersRequest { string team_id = 1; string workspace_id = 2; }
message ListTeamMembersResponse { repeated Member members = 1; }

message GetCustomViewRequest { string view_id = 1; string model = 2; string workspace_id = 3; }

message ListWorkflowStatesRequest { string team_id = 1; string workspace_id = 2; }
message ListWorkflowStatesResponse { repeated WorkflowState states = 1; }
```

### 3. Add Linear-specific fields to `CreateIssueRequest`

`CreateIssueRequest` (TASK-096) reserved field numbers 13/14 for this —
find the comment `// fields 13/14 (team_id/state_id) reserved for
SOL-016/TASK-102 (Linear-only)` inside `CreateIssueRequest` and replace it
with:

```protobuf
  string team_id = 13;  // Linear: replaces project_key as the primary grouping
  string state_id = 14; // Linear: initial workflow state, no Jira equivalent (Jira issues start in a fixed default status)
```

`buf breaking` note: purely additive — new RPCs, new messages, two new
`CreateIssueRequest` fields at previously-reserved-but-unused numbers. No
existing field renumbered.

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
