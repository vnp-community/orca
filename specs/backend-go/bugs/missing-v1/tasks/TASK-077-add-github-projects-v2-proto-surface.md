# TASK-077: Add `github.project.*` (GitHub Projects v2) proto sub-surface

**From Solution:** SOL-012 (Design — Proto additions, shape 3)
**Priority:** P1
**Service:** `scm-integration-service`
**File:** `backend-go/proto/orca/scmintegration/v1/scmintegration.proto`
**Depends on:** TASK-071 (same file; apply after to avoid a service-block merge conflict)
**Status:** `[ ]` TODO

---

## Context

GitHub Projects v2 is GraphQL-only (no REST equivalent) and its items carry
per-project-configurable custom fields — a structurally different shape from
`Issue`/`PullRequest`'s fixed-field pattern. Per SOL-012, this is a
**GitHub-only** sub-surface, not routed through the provider-generic
`ScmProvider` enum: no other supported provider (GitLab/Bitbucket/Azure
DevOps/Gitea) has a Projects v2 equivalent. 16 new RPCs under one
`github.project.*`-mirroring block (the frontend's 17th method,
`resolveRef`'s underlying item read, is folded into `ResolveProjectRef`/
`ViewProjectTable`'s own implementation, not a separate RPC — see SOL-012's
signature table note). All additive; `buf breaking` stays clean.

---

## Changes to make

**File:** `backend-go/proto/orca/scmintegration/v1/scmintegration.proto`

### Step 1: Add the `google.protobuf.Empty` import

Find:

```protobuf
syntax = "proto3";

package orca.scmintegration.v1;

option go_package = "github.com/stablyai/orca-go/proto/gen/go/orca/scmintegration/v1;scmintegrationv1";
```

Replace with:

```protobuf
syntax = "proto3";

package orca.scmintegration.v1;

import "google/protobuf/empty.proto";

option go_package = "github.com/stablyai/orca-go/proto/gen/go/orca/scmintegration/v1;scmintegrationv1";
```

### Step 2: Add RPCs to the `ScmIntegrationService` service block

Find (added by TASK-072):

```protobuf
  rpc ResolveRepoSlug(ResolveRepoSlugRequest) returns (ResolveRepoSlugResponse);
}
```

Replace with:

```protobuf
  rpc ResolveRepoSlug(ResolveRepoSlugRequest) returns (ResolveRepoSlugResponse);

  // GitHub Projects v2 (GraphQL-only, GitHub-specific — no other provider
  // has an equivalent). tenant_id resolves the same per-tenant GitHub OAuth
  // token as every other RPC on this service; there is no separate
  // Projects-v2 auth. See SOL-012 "Design — Proto additions, shape 3".
  rpc ListAccessibleProjects(ListAccessibleProjectsRequest) returns (ListAccessibleProjectsResponse);
  rpc ResolveProjectRef(ResolveProjectRefRequest) returns (ResolveProjectRefResponse);
  rpc ListProjectViews(ListProjectViewsRequest) returns (ListProjectViewsResponse);
  rpc ViewProjectTable(ViewProjectTableRequest) returns (ViewProjectTableResponse);
  rpc UpdateProjectItemField(UpdateProjectItemFieldRequest) returns (ProjectItem);
  rpc ClearProjectItemField(ClearProjectItemFieldRequest) returns (ProjectItem);
  rpc GetWorkItemDetailsBySlug(GetWorkItemDetailsBySlugRequest) returns (WorkItemDetails);
  rpc UpdateIssueBySlug(UpdateIssueBySlugRequest) returns (WorkItemDetails);
  rpc UpdatePullRequestBySlug(UpdatePullRequestBySlugRequest) returns (WorkItemDetails);
  rpc UpdateIssueTypeBySlug(UpdateIssueTypeBySlugRequest) returns (WorkItemDetails);
  rpc ListIssueTypesBySlug(ListIssueTypesBySlugRequest) returns (ListIssueTypesBySlugResponse);
  rpc ListAssignableUsersBySlug(ListAssignableUsersBySlugRequest) returns (ListAssignableUsersBySlugResponse);
  rpc ListLabelsBySlug(ListLabelsBySlugRequest) returns (ListLabelsBySlugResponse);
  rpc AddIssueCommentBySlug(AddIssueCommentBySlugRequest) returns (ProjectComment);
  rpc UpdateIssueCommentBySlug(UpdateIssueCommentBySlugRequest) returns (ProjectComment);
  rpc DeleteIssueCommentBySlug(DeleteIssueCommentBySlugRequest) returns (google.protobuf.Empty);
}
```

### Step 3: Append new messages

Add to the bottom of the file:

```protobuf
// ── GitHub Projects v2 ──────────────────────────────────────────────────

// ProjectFieldValue is a generic key/value field write — Projects v2 fields
// are per-project-defined (text/number/date/single-select/iteration); the
// GraphQL mutation itself (updateProjectV2ItemFieldValue) takes a typed
// union, so the adapter (not this proto) picks the right GraphQL input
// shape from this string+kind pair.
message ProjectFieldValue {
  string field_id = 1;
  string kind = 2;  // "text" | "number" | "date" | "single_select" | "iteration"
  string value = 3; // string-encoded; adapter parses per kind
}

message ProjectItem {
  string id = 1;
  string title = 2;
  string content_type = 3; // "issue" | "pull_request" | "draft_issue"
  string content_url = 4;
  repeated ProjectFieldValue fields = 5;
}

message Project {
  string id = 1;
  string slug = 2;   // "owner/number", canonical addressing form
  string title = 3;
  int32 number = 4;
  string owner = 5;
  string url = 6;
}

message ProjectView {
  string id = 1;
  string name = 2;
  string layout = 3; // "table" | "board" | "roadmap"
}

message IssueType {
  string id = 1;
  string name = 2;
  string description = 3;
}

message AssignableUser {
  string login = 1;
  string name = 2;
  string avatar_url = 3;
}

message Label {
  string name = 1;
  string color = 2;
  string description = 3;
}

message ProjectComment {
  string id = 1;
  string body = 2;
  string author = 3;
  string url = 4;
}

// "BySlug" methods all take an item_slug ("owner/repo#number") + tenant_id —
// mirrors the frontend's own by-slug addressing scheme (BUG-012's finding:
// no by-slug addressing scheme existed server-side; this is where it's
// added). tenant_id resolves credentials the same way as every other RPC.
message WorkItemDetails {
  string slug = 1;
  string title = 2;
  string body = 3;
  string state = 4;
  string url = 5;
  repeated ProjectFieldValue fields = 6;
}

message ListAccessibleProjectsRequest {
  string tenant_id = 1;
}
message ListAccessibleProjectsResponse {
  repeated Project projects = 1;
}

message ResolveProjectRefRequest {
  string tenant_id = 1;
  string owner = 2;
  int32 number = 3;
}
message ResolveProjectRefResponse {
  string slug = 1;
  Project project = 2;
}

message ListProjectViewsRequest {
  string tenant_id = 1;
  string project_slug = 2;
}
message ListProjectViewsResponse {
  repeated ProjectView views = 1;
}

message ViewProjectTableRequest {
  string tenant_id = 1;
  string project_slug = 2;
  string view_id = 3;    // empty = the project's default view
  string page_token = 4;
  int32 page_size = 5;
}
message ViewProjectTableResponse {
  repeated ProjectItem items = 1;
  string next_page_token = 2;
}

message UpdateProjectItemFieldRequest {
  string tenant_id = 1;
  string project_slug = 2;
  string item_id = 3;
  ProjectFieldValue field = 4;
}

message ClearProjectItemFieldRequest {
  string tenant_id = 1;
  string project_slug = 2;
  string item_id = 3;
  string field_id = 4;
}

message GetWorkItemDetailsBySlugRequest {
  string tenant_id = 1;
  string item_slug = 2;
}

message UpdateIssueBySlugRequest {
  string tenant_id = 1;
  string item_slug = 2;
  optional string title = 3;
  optional string body = 4;
  optional string state = 5;
  repeated string add_labels = 6;
  repeated string remove_labels = 7;
}

message UpdatePullRequestBySlugRequest {
  string tenant_id = 1;
  string item_slug = 2;
  optional string title = 3;
  optional string body = 4;
  optional string state = 5;
}

message UpdateIssueTypeBySlugRequest {
  string tenant_id = 1;
  string item_slug = 2;
  string issue_type = 3;
}

message ListIssueTypesBySlugRequest {
  string tenant_id = 1;
  string item_slug = 2;
}
message ListIssueTypesBySlugResponse {
  repeated IssueType issue_types = 1;
}

message ListAssignableUsersBySlugRequest {
  string tenant_id = 1;
  string item_slug = 2;
}
message ListAssignableUsersBySlugResponse {
  repeated AssignableUser users = 1;
}

message ListLabelsBySlugRequest {
  string tenant_id = 1;
  string item_slug = 2;
}
message ListLabelsBySlugResponse {
  repeated Label labels = 1;
}

message AddIssueCommentBySlugRequest {
  string tenant_id = 1;
  string item_slug = 2;
  string body = 3;
}

message UpdateIssueCommentBySlugRequest {
  string tenant_id = 1;
  string item_slug = 2;
  string comment_id = 3;
  string body = 4;
}

message DeleteIssueCommentBySlugRequest {
  string tenant_id = 1;
  string item_slug = 2;
  string comment_id = 3;
}
```

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

Expected: clean build, `buf breaking` reports no breaking changes (only additions).
