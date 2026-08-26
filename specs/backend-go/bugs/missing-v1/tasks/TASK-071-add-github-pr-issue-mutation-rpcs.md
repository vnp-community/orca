# TASK-071: Add GitHub PR/issue mutation RPCs to `scmintegration.proto`

**From Solution:** SOL-012 (Design — Proto additions, shape 1)
**Priority:** P1 — everything else in SOL-012/013/014 depends on generated stubs from this proto file
**Service:** `scm-integration-service`
**File:** `backend-go/proto/orca/scmintegration/v1/scmintegration.proto`
**Depends on:** none
**Status:** `[x]` DONE — implemented in worktree `agent-aac2382028c6ce920` (branch `worktree-agent-aac2382028c6ce920`), **committed** as `ce750c490`. `go build`/`go vet`/`gofmt -l` clean, `buf generate`/`buf breaking` clean (additive-only). Pending merge to main + one-line RegisterRealChannels/main.go wiring.

---

## Context

`github.mergePR`, `github.requestPRReviewers`, `github.removePRReviewers`,
`github.setPRAutoMerge`, and `github.updateIssue` have no backing RPC today.
This task adds 5 new RPCs (`MergePullRequest`, `RequestPullRequestReviewers`,
`RemovePullRequestReviewers`, `SetPullRequestAutoMerge`, `UpdateIssue`) plus
an additive `number` field on `Issue`/`PullRequest` — GitHub addresses
everything by repo-scoped `number`, which neither message currently carries.
All additions are additive only; `buf breaking` stays clean.

---

## Changes to make

**File:** `backend-go/proto/orca/scmintegration/v1/scmintegration.proto`

### Step 1: Add RPCs to the `ScmIntegrationService` service block

Find:

```protobuf
  rpc RevokeAuth(RevokeAuthRequest) returns (RevokeAuthResponse);
}
```

Replace with:

```protobuf
  rpc RevokeAuth(RevokeAuthRequest) returns (RevokeAuthResponse);

  // GitHub PR/issue mutations — github.mergePR / github.requestPRReviewers /
  // github.removePRReviewers / github.setPRAutoMerge / github.updateIssue.
  // See SOL-012 "Design — Proto additions, shape 1".
  rpc MergePullRequest(MergePullRequestRequest) returns (MergePullRequestResponse);
  rpc RequestPullRequestReviewers(RequestPullRequestReviewersRequest) returns (PullRequest);
  rpc RemovePullRequestReviewers(RemovePullRequestReviewersRequest) returns (PullRequest);
  rpc SetPullRequestAutoMerge(SetPullRequestAutoMergeRequest) returns (PullRequest);
  rpc UpdateIssue(UpdateIssueRequest) returns (Issue);
}
```

### Step 2: Add `number` field to `Issue` and `PullRequest`

Find:

```protobuf
message Issue {
  string id = 1;
  string title = 2;
  string state = 3;
  string url = 4;
}
```

Replace with:

```protobuf
message Issue {
  string id = 1;
  string title = 2;
  string state = 3;
  string url = 4;
  // number is GitHub's repo-scoped issue number (e.g. #42) — every REST
  // mutation below addresses issues/PRs by number, not the opaque id above.
  // Additive field; existing ListIssues callers are unaffected.
  int32 number = 5;
}
```

Find:

```protobuf
message PullRequest {
  string id = 1;
  string url = 2;
  string state = 3;
}
```

Replace with:

```protobuf
message PullRequest {
  string id = 1;
  string url = 2;
  string state = 3;
  // number is GitHub's repo-scoped PR number — see Issue.number's doc
  // comment for why this is additive, not a breaking rename.
  int32 number = 4;
}
```

### Step 3: Append new request/response messages

Add to the bottom of the file:

```protobuf
message MergePullRequestRequest {
  string tenant_id = 1;
  ScmProvider provider = 2;
  string repo = 3;
  int32 number = 4;
  string merge_method = 5; // "merge" | "squash" | "rebase"
  string commit_title = 6;
  string commit_message = 7;
}
message MergePullRequestResponse {
  PullRequest pull_request = 1;
  bool merged = 2;
  string sha = 3;
}

message RequestPullRequestReviewersRequest {
  string tenant_id = 1;
  ScmProvider provider = 2;
  string repo = 3;
  int32 number = 4;
  repeated string reviewer_logins = 5;
  repeated string team_slugs = 6;
}
message RemovePullRequestReviewersRequest {
  string tenant_id = 1;
  ScmProvider provider = 2;
  string repo = 3;
  int32 number = 4;
  repeated string reviewer_logins = 5;
}

message SetPullRequestAutoMergeRequest {
  string tenant_id = 1;
  ScmProvider provider = 2;
  string repo = 3;
  int32 number = 4;
  bool enabled = 5;
  string merge_method = 6;
}

message UpdateIssueRequest {
  string tenant_id = 1;
  ScmProvider provider = 2;
  string repo = 3;
  int32 number = 4;
  optional string title = 5;
  optional string body = 6;
  optional string state = 7;
  repeated string add_labels = 8;
  repeated string remove_labels = 9;
  repeated string assignees = 10;
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
