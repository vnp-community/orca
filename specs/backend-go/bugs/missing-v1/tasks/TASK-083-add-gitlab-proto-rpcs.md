# TASK-083: Add `gitlab.*` RPCs to `scmintegration.proto`

**From Solution:** SOL-013 (Design — Proto additions)
**Priority:** P1
**Service:** `scm-integration-service`
**File:** `backend-go/proto/orca/scmintegration/v1/scmintegration.proto`
**Depends on:** TASK-077 (same file; apply after to avoid a service-block merge conflict)
**Status:** `[ ]` TODO

---

## Context

`gitlab.listMRs`, `gitlab.resolveMRDiscussion`, and `gitlab.workItemDetails`
have no backing RPC. All three are deliberately **GitLab-only** (no
`provider` field, unlike every other RPC in this file) — they model
GitLab-specific concepts (`iid`, discussions, merge-request-vs-issue
addressing) that don't generalize across providers, mirroring SOL-012's
GitHub Projects v2 RPCs being GitHub-only for the same reason.
`gitlab.rateLimit` stays on the existing provider-generic
`GetRateLimitStatus` — no proto change needed for it.

---

## Changes to make

**File:** `backend-go/proto/orca/scmintegration/v1/scmintegration.proto`

### Step 1: Add RPCs to the `ScmIntegrationService` service block

Find (added by TASK-077):

```protobuf
  rpc DeleteIssueCommentBySlug(DeleteIssueCommentBySlugRequest) returns (google.protobuf.Empty);
}
```

Replace with:

```protobuf
  rpc DeleteIssueCommentBySlug(DeleteIssueCommentBySlugRequest) returns (google.protobuf.Empty);

  // GitLab-specific RPCs — gitlab.listMRs / gitlab.resolveMRDiscussion /
  // gitlab.workItemDetails. Deliberately NOT parameterized by ScmProvider —
  // these model GitLab-only concepts (iid, discussions). See SOL-013.
  rpc ListMergeRequests(ListMergeRequestsRequest) returns (ListMergeRequestsResponse);
  rpc ResolveMergeRequestDiscussion(ResolveMergeRequestDiscussionRequest) returns (MergeRequestDiscussion);
  rpc GetWorkItemDetails(GetWorkItemDetailsRequest) returns (WorkItemDetailsGitLab);
}
```

### Step 2: Append new messages

Add to the bottom of the file:

```protobuf
// ── GitLab-specific ─────────────────────────────────────────────────────

message MergeRequest {
  string id = 1;
  string url = 2;
  string state = 3;              // "opened" | "closed" | "merged" | "locked"
  int32 iid = 4;                 // project-scoped internal ID, what the UI/URLs use
  string title = 5;
  string source_branch = 6;
  string target_branch = 7;
  bool draft = 8;
  int32 discussion_count = 9;
  int32 unresolved_discussion_count = 10;
  string merge_status = 11;      // GitLab's can_be_merged / mergeable / etc.
}

message ListMergeRequestsRequest {
  string tenant_id = 1;
  string repo = 2;               // GitLab project path ("group/project")
  string state = 3;               // "opened" | "closed" | "merged" | "all"; empty = "opened"
  string source_branch = 4;       // optional filter
}
message ListMergeRequestsResponse {
  repeated MergeRequest merge_requests = 1;
}

message MergeRequestDiscussion {
  string id = 1;
  bool resolved = 2;
  string resolved_by = 3;
}
message ResolveMergeRequestDiscussionRequest {
  string tenant_id = 1;
  string repo = 2;
  int32 merge_request_iid = 3;
  string discussion_id = 4;
  bool resolved = 5;
}

message GetWorkItemDetailsRequest {
  string tenant_id = 1;
  string repo = 2;
  int32 iid = 3;
  string item_type = 4;          // "merge_request" | "issue"
}
message WorkItemDetailsGitLab {
  string id = 1;
  int32 iid = 2;
  string item_type = 3;
  string title = 4;
  string body = 5;
  string state = 6;
  string url = 7;
  repeated string labels = 8;
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
