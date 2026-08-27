# TASK-CR-05-01: Add draft/linked-issue fields and `SuggestPullRequestReviewers` RPC to `scmintegration.proto`

**From Solution:** SOL-CR-05
**Priority:** P0 — every other task in this set depends on generated stubs from this
**Service:** `scm-integration-service`
**File:** `backend-go/proto/orca/scmintegration/v1/scmintegration.proto`
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

BUG-CR-05's draft-PR option (BR-CR-20), CODEOWNERS reviewer suggestion
(BR-CR-18), and linked-issue auto-update (BR-CR-19) need new wire fields
plus one new RPC. Additive only, so `buf breaking` stays clean.

## Changes to make

In the `ScmIntegrationService` service block, add (near `CreatePullRequest`):

```protobuf
rpc SuggestPullRequestReviewers(SuggestPullRequestReviewersRequest) returns (SuggestPullRequestReviewersResponse);
```

Update `CreatePullRequestRequest`:

```protobuf
message CreatePullRequestRequest {
  string tenant_id = 1;
  ScmProvider provider = 2;
  string repo = 3;
  string title = 4;
  string body = 5;
  string head_branch = 6;
  string base_branch = 7;
  string request_id = 8;
  bool draft = 9;                          // NEW — BR-CR-20
  optional int32 linked_issue_number = 10; // NEW — BR-CR-19
}
```

Update `PullRequest`:

```protobuf
message PullRequest {
  string id = 1;
  string url = 2;
  string state = 3;
  int32 number = 4;
  bool draft = 5; // NEW — echoes the provider's actual draft state; a
                  // provider without draft support (Bitbucket) never
                  // returns true even when requested
}
```

Update `CreatePullRequestResponse`:

```protobuf
message CreatePullRequestResponse {
  PullRequest pull_request = 1;
  // NEW — set only when linked_issue_number was provided and the PR was
  // created successfully but the issue update itself failed. The PR is
  // NOT rolled back for this.
  string linked_issue_update_error = 2;
}
```

Append new messages at the bottom of the file:

```protobuf
message SuggestPullRequestReviewersRequest {
  string tenant_id = 1;
  ScmProvider provider = 2;
  string repo = 3;
  string base_ref = 4;               // CODEOWNERS is read from base_ref, matching GitHub's own resolution rule
  repeated string changed_files = 5; // caller-supplied — see TASK-CR-05-08 for why
}

message SuggestPullRequestReviewersResponse {
  repeated string reviewer_logins = 1;
  repeated string team_slugs = 2;
  bool codeowners_found = 3; // false = no CODEOWNERS file at any canonical path; empty suggestion is not an error
}
```

`draft` defaults to `false` (proto3 zero value) — a deliberate,
backward-compatible default for existing callers; BR-CR-20 requires the
*option* to exist, not that draft become the default.

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
