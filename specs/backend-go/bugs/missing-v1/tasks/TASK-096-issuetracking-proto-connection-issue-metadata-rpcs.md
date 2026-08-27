# TASK-096: Extend `issuetracking.proto` with connection, issue-CRUD, and metadata RPCs (Jira scope)

**From Solution:** SOL-015
**Priority:** P0 — everything else in SOL-015/SOL-016 depends on generated stubs from this
**Service:** `issue-tracking-service`
**File:** `backend-go/proto/orca/issuetracking/v1/issuetracking.proto`
**Depends on:** none
**Status:** `[x]` DONE — implemented in worktree `agent-a412325f0d1276bb5` (branch `worktree-agent-a412325f0d1276bb5`), **committed** as `c29ca9e6a`. `go build`/`go vet`/`buf generate`/`buf breaking` clean. Pending merge.

---

## Context

`issuetracking.proto` today has 3 RPCs (`ListIssues`, `CreateIssue`,
`LinkIssue`) backing a thin `Issue{id,title,state,url}` message. BUG-015
found 19 `jira.*` frontend channels with no backend at all. SOL-015 maps
them onto 18 new RPCs (one `jira.*` method, `disconnect`, folds into
`Disconnect`; `status`/`testConnection`/etc. each get their own RPC — see
SOL-015's mapping table) plus 3 RPCs the TDD never sketched
(`ListPriorities`, `ListTransitions`, `GetProjectStatusOrder`), flagged
there as scope additions.

This task adds every RPC/message SOL-015 needs. It is additive only — no
existing field is removed or renumbered on `ListIssuesRequest`/
`CreateIssueRequest`/`Issue`, so `buf breaking` stays clean. SOL-016
(TASK-102) appends further (Linear-only) RPCs/messages on top of this same
file afterward — do not implement Linear-specific messages here.

## Changes to make

Replace the full contents of
`backend-go/proto/orca/issuetracking/v1/issuetracking.proto` with:

```protobuf
syntax = "proto3";

package orca.issuetracking.v1;

import "google/protobuf/empty.proto";
import "google/protobuf/timestamp.proto";

option go_package = "github.com/stablyai/orca-go/proto/gen/go/orca/issuetracking/v1;issuetrackingv1";

// IssueTrackingService: Jira/Linear direct API integration (already correct
// shape in TS, faithful port). See specs/backend-go/services/issue-tracking-service.md.
service IssueTrackingService {
  // ── existing, kept for compatibility ──────────────────────────────
  rpc ListIssues(ListIssuesRequest) returns (ListIssuesResponse);   // fields extended below
  rpc CreateIssue(CreateIssueRequest) returns (CreateIssueResponse); // fields extended below
  rpc LinkIssue(LinkIssueRequest) returns (LinkIssueResponse);       // == TDD's LinkIssueToTask

  // ── connection mgmt (new, SOL-015) ──────────────────────────────────
  rpc Connect(ConnectRequest) returns (ConnectionStatus);
  rpc Disconnect(DisconnectRequest) returns (google.protobuf.Empty);
  rpc SelectWorkspace(SelectWorkspaceRequest) returns (ConnectionStatus);
  rpc GetConnectionStatus(GetConnectionStatusRequest) returns (ConnectionStatus);
  rpc TestConnection(TestConnectionRequest) returns (TestConnectionResult);

  // ── issue querying/mutation beyond ListIssues/CreateIssue (new, SOL-015) ────
  rpc SearchIssues(SearchIssuesRequest) returns (SearchIssuesResponse);
  rpc GetIssue(GetIssueRequest) returns (Issue);
  rpc UpdateIssue(UpdateIssueRequest) returns (Issue);
  rpc AddIssueComment(AddIssueCommentRequest) returns (IssueComment);
  rpc ListIssueComments(ListIssueCommentsRequest) returns (ListIssueCommentsResponse);

  // ── project/workflow metadata (new, SOL-015) ─────────────────────────────
  rpc ListProjects(ListProjectsRequest) returns (ListProjectsResponse);
  rpc ListIssueTypes(ListIssueTypesRequest) returns (ListIssueTypesResponse); // Jira only
  rpc ListCreateFields(ListCreateFieldsRequest) returns (ListCreateFieldsResponse);
  rpc ListAssignableUsers(ListAssignableUsersRequest) returns (ListAssignableUsersResponse);

  // ── scope additions beyond the TDD (flagged, not silently skipped) ──
  rpc ListPriorities(ListPrioritiesRequest) returns (ListPrioritiesResponse);
  rpc ListTransitions(ListTransitionsRequest) returns (ListTransitionsResponse);
  rpc GetProjectStatusOrder(GetProjectStatusOrderRequest) returns (GetProjectStatusOrderResponse);

  // ── SOL-016 (linear.*) appends its own RPCs here — see TASK-102 ──────
}

enum IssueProvider {
  ISSUE_PROVIDER_UNSPECIFIED = 0;
  ISSUE_PROVIDER_JIRA = 1;
  ISSUE_PROVIDER_LINEAR = 2;
}

// Workspace unifies Jira's "site" and Linear's "workspace" — one connected
// account can have more than one (JiraConnectionStatus.sites in the
// frontend), so ConnectionStatus carries a list.
message Workspace {
  string id = 1;
  string name = 2;
  string url = 3; // Jira site base URL; empty for Linear
}

message ConnectionStatus {
  bool connected = 1;
  string viewer_id = 2;
  string viewer_display_name = 3;
  string viewer_email = 4;
  repeated Workspace workspaces = 5;
  string active_workspace_id = 6;
  string selected_workspace_id = 7; // "" | "all" | a specific id — see JiraSiteSelection
  string credential_error = 8;      // set when a stored credential exists but resolution/decrypt failed
}

message ConnectRequest {
  IssueProvider provider = 1;
  string site_url = 2; // Jira only
  string email = 3;    // Jira only
  string token = 4;    // Jira API token, or Linear personal API key / OAuth access token
}

message DisconnectRequest {
  IssueProvider provider = 1;
  string workspace_id = 2; // optional — disconnect one site/workspace; empty disconnects all
}

message SelectWorkspaceRequest {
  IssueProvider provider = 1;
  string workspace_id = 2; // "" | "all" | a specific workspace id
}

message GetConnectionStatusRequest {
  IssueProvider provider = 1;
}

message TestConnectionRequest {
  IssueProvider provider = 1;
  string workspace_id = 2; // optional
}

message TestConnectionResult {
  bool ok = 1;
  string error = 2; // set when ok is false
}

message Issue {
  string id = 1;               // provider-agnostic id (was already here)
  string provider_issue_id = 2; // raw provider id/key, for round-tripping mutations
  string key = 3;               // "PROJ-123" (Jira) / "ENG-42" (Linear identifier)
  string title = 4;
  string description_markdown = 5;
  string state = 6;             // kept for back-compat with existing callers
  WorkflowState workflow_state = 7;
  string url = 8;
  Project project = 9;
  IssueType issue_type = 10;    // Jira only; unset for Linear
  repeated string labels = 11;
  UserRef assignee = 12;
  UserRef reporter = 13;        // Jira only
  Priority priority = 14;
  string custom_fields_json = 15; // JSON-encoded map — Jira create/edit custom fields
  google.protobuf.Timestamp created_at = 16;
  google.protobuf.Timestamp updated_at = 17;
}

message Project { string id = 1; string key = 2; string name = 3; string workspace_id = 4; }
message IssueType { string id = 1; string name = 2; bool subtask = 3; }
message WorkflowState { string id = 1; string name = 2; string category = 3; } // todo|in_progress|done|cancelled
message UserRef { string id = 1; string display_name = 2; string email = 3; string avatar_url = 4; }
message Priority { string id = 1; string name = 2; }
message IssueComment { string id = 1; string body_markdown = 2; UserRef author = 3; google.protobuf.Timestamp created_at = 4; google.protobuf.Timestamp updated_at = 5; }

message ListIssuesRequest {
  string tenant_id = 1;
  IssueProvider provider = 2;
  string project_key = 3;
  // New, additive:
  string filter_json = 4; // JSON-encoded provider-specific filter object
  int32 limit = 5;
  string workspace_id = 6; // site/workspace selector
}

message ListIssuesResponse {
  repeated Issue issues = 1;
}

message SearchIssuesRequest {
  IssueProvider provider = 1;
  string query = 2; // Jira: JQL string. Linear: free-text/filter query.
  int32 limit = 3;
  string workspace_id = 4;
}

message SearchIssuesResponse {
  repeated Issue issues = 1;
}

message GetIssueRequest {
  IssueProvider provider = 1;
  string issue_id = 2; // provider_issue_id/key — see Issue.provider_issue_id
  string workspace_id = 3;
}

message CreateIssueRequest {
  string tenant_id = 1;
  IssueProvider provider = 2;
  string project_key = 3;
  string title = 4;
  string description = 5;
  // New, additive:
  string issue_type_id = 6;   // Jira
  string assignee_id = 7;
  string priority_id = 8;
  repeated string label_ids = 9;
  string parent_issue_id = 10; // Linear sub-issue
  string custom_fields_json = 11; // arbitrary Jira create-field bag, keyed by field key
  string workspace_id = 12;
  // fields 13/14 (team_id/state_id) reserved for SOL-016/TASK-102 (Linear-only)
}

message CreateIssueResponse {
  Issue issue = 1;
}

message UpdateIssueRequest {
  IssueProvider provider = 1;
  string issue_id = 2;
  string title = 3;             // empty = leave unchanged
  string description = 4;       // empty = leave unchanged
  string assignee_id = 5;
  string priority_id = 6;
  repeated string label_ids = 7;
  string workflow_state_id = 8; // == jira.updateIssue's transition target / linear's stateId
  string custom_fields_json = 9;
  string workspace_id = 10;
}

message AddIssueCommentRequest {
  IssueProvider provider = 1;
  string issue_id = 2;
  string body_markdown = 3;
  string workspace_id = 4;
}

message ListIssueCommentsRequest {
  IssueProvider provider = 1;
  string issue_id = 2;
  string workspace_id = 3;
}

message ListIssueCommentsResponse {
  repeated IssueComment comments = 1;
}

message ListProjectsRequest {
  IssueProvider provider = 1;
  string workspace_id = 2;
}

message ListProjectsResponse {
  repeated Project projects = 1;
}

message ListIssueTypesRequest {
  string project_id_or_key = 1;
  string workspace_id = 2;
}

message ListIssueTypesResponse {
  repeated IssueType issue_types = 1;
}

message CreateField {
  string key = 1;
  string name = 2;
  bool required = 3;
  string schema_type = 4;
  string schema_items = 5;
  string schema_custom = 6;
  string allowed_values_json = 7; // JSON array — heterogeneous per-field shape, not worth a message
}

message ListCreateFieldsRequest {
  string project_id_or_key = 1;
  string issue_type_id = 2;
  string workspace_id = 3;
}

message ListCreateFieldsResponse {
  repeated CreateField fields = 1;
}

message ListAssignableUsersRequest {
  IssueProvider provider = 1;
  string project_id_or_key = 2; // Jira
  string issue_id = 3;          // optional — narrows to users assignable to this specific issue
  string workspace_id = 4;
}

message ListAssignableUsersResponse {
  repeated UserRef users = 1;
}

message ListPrioritiesRequest {
  string workspace_id = 1;
}

message ListPrioritiesResponse {
  repeated Priority priorities = 1;
}

message Transition {
  string id = 1;
  string name = 2;
  WorkflowState to = 3;
}

message ListTransitionsRequest {
  string issue_id = 1;
  string workspace_id = 2;
}

message ListTransitionsResponse {
  repeated Transition transitions = 1;
}

message GetProjectStatusOrderRequest {
  string project_id_or_key = 1;
  string workspace_id = 2;
}

message GetProjectStatusOrderResponse {
  // status_ids_by_column mirrors JiraProjectStatusOrder.statusIdsByColumn —
  // Jira's per-project Kanban column grouping, a list of columns, each a
  // list of status ids in that column.
  repeated StatusIDList status_ids_by_column = 1;
}

message StatusIDList {
  repeated string status_ids = 1;
}

// LinkIssue publishes an orca.issuetracking.link.created event rather than
// writing directly into task-service/project-service's data (async, per
// architecture/08-inter-service-communication.md).
message LinkIssueRequest {
  string issue_id = 1;
  string task_id = 2;
}

message LinkIssueResponse {}
```

`buf breaking` note: `ListIssuesRequest.filter_json`/`limit`/`workspace_id`,
`CreateIssueRequest`'s new fields (6-12), and `Issue`'s new fields (2,
5-17) are appended at previously-unused field numbers — `tenant_id`/
`provider`/`project_key`/`title`/`description` on the request messages and
`id`/`title`/`state`/`url` on `Issue` keep their original numbers, so
existing callers (the `/v1/issues` REST proxy, `httpgateway`) keep
compiling unmodified.

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
