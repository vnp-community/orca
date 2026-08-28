# TASK-PI-01-01: Add issue filter fields + comment/linked-PR RPCs to `scmintegration.proto`

**From Solution:** SOL-PI-01
**Priority:** P0 — everything else in this solution depends on generated stubs from this
**Service:** `scm-integration-service`
**File:** `backend-go/proto/orca/scmintegration/v1/scmintegration.proto`
**Depends on:** none
**Status:** `[x] DONE — IssueFilter/ListIssueCommentsBySlug/GetLinkedPullRequestsForIssue added to scmintegration.proto, buf generate clean.`

---

## Context

BUG-PI-01 found `ListIssuesRequest` (currently `tenant_id`/`provider`/`repo`
only, proto lines ~112-116) carries no filter fields even though
`usecase.IssueFilter.State` already exists unused, and that no RPC exists to
list an issue's comments or its linked PRs. This task adds the proto surface
only — additive, so `buf breaking` stays clean. Naming note (grounding
correction vs. SOL-PI-01's sketch): the new comment-list request field is
named `item_slug`, matching every other `*BySlug` request in this file
(`GetWorkItemDetailsBySlugRequest.item_slug`), not `slug`; and the response
reuses the existing `ProjectComment` message (id/body/author/url, already
returned by `AddIssueCommentBySlug`) instead of inventing a new
`IssueComment` type.

## Changes to make

In the `ScmIntegrationService` service block, add two RPCs right after
`rpc ListIssues(...)`:

```protobuf
  rpc ListIssueCommentsBySlug(ListIssueCommentsBySlugRequest) returns (ListIssueCommentsBySlugResponse);
  rpc GetLinkedPullRequestsForIssue(GetLinkedPullRequestsForIssueRequest) returns (GetLinkedPullRequestsForIssueResponse);
```

Replace the existing `ListIssuesRequest`/`ListIssuesResponse` messages:

```protobuf
message IssueFilter {
  string state = 1;       // "open" | "closed" | "all" — default "open"
  string assignee = 2;    // GitHub login / GitLab username; "" = unfiltered
  repeated string labels = 3;
  string milestone = 4;   // milestone title or number-as-string
}

message ListIssuesRequest {
  string tenant_id = 1;
  ScmProvider provider = 2;
  string repo = 3;
  IssueFilter filter = 4;      // NEW — was previously absent
  bool force_refresh = 5;      // NEW — bypasses the 5-minute cache
}

message ListIssuesResponse {
  repeated Issue issues = 1;
  bool from_cache = 2;         // NEW
  int64 cached_at_unix_ms = 3; // NEW
}
```

Add new messages (append near `ProjectComment`/`WorkItemDetails`):

```protobuf
// ListIssueCommentsBySlug completes the *BySlug comment RPC group —
// AddIssueCommentBySlug/UpdateIssueCommentBySlug/DeleteIssueCommentBySlug
// already exist with no way to read the thread back.
message ListIssueCommentsBySlugRequest {
  string tenant_id = 1;
  string item_slug = 2; // matches every other *BySlug request's field name
}
message ListIssueCommentsBySlugResponse {
  repeated ProjectComment comments = 1; // reuses AddIssueCommentBySlug's existing comment shape
}

// GetLinkedPullRequestsForIssue has no *BySlug precedent — provider-generic
// like ListIssues. A provider with no cheap "linked PRs" query sets
// capability_unsupported=true and returns an empty list, never an RPC error
// (same degrade pattern as GetBoardView's ErrCapabilityUnsupported).
message GetLinkedPullRequestsForIssueRequest {
  string tenant_id = 1;
  ScmProvider provider = 2;
  string repo = 3;
  int32 issue_number = 4;
}
message GetLinkedPullRequestsForIssueResponse {
  repeated PullRequest pull_requests = 1;
  bool capability_unsupported = 2;
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
