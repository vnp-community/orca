# TASK-087: Add `CheckHostedReviewEligibility` RPC to `scmintegration.proto`

**From Solution:** SOL-014 (Design — Proto addition: `CheckHostedReviewEligibility`)
**Priority:** P1
**Service:** `scm-integration-service`
**File:** `backend-go/proto/orca/scmintegration/v1/scmintegration.proto`
**Depends on:** TASK-072 (uses `GetPullRequestForBranch`/`PullRequest`), TASK-083 (same file; apply after to avoid a service-block merge conflict)
**Status:** `[x]` DONE (verified — `buf generate`/`buf breaking` clean, `go build ./proto/...` clean)

---

## Context

`hostedReview.create`/`hostedReview.forBranch` need no new RPC — they wire
directly onto `CreatePullRequest`/`GetPullRequestForBranch` (TASK-089).
`hostedReview.getCreationEligibility` has no backing RPC anywhere; this adds
`CheckHostedReviewEligibility`, already named and scoped in
`scm-integration-service.md` §3 — a gap-closing task against an
already-specified RPC, not a new invention. It composes 3 signals
(`GetAuthStatus`, a new `BranchExists` provider-port check, TASK-072's
`GetPullRequestForBranch`) — see TASK-088 for the usecase.

---

## Changes to make

**File:** `backend-go/proto/orca/scmintegration/v1/scmintegration.proto`

### Step 1: Add RPC to the `ScmIntegrationService` service block

Find (added by TASK-083):

```protobuf
  rpc GetWorkItemDetails(GetWorkItemDetailsRequest) returns (WorkItemDetailsGitLab);
}
```

Replace with:

```protobuf
  rpc GetWorkItemDetails(GetWorkItemDetailsRequest) returns (WorkItemDetailsGitLab);

  // CheckHostedReviewEligibility — hostedReview.getCreationEligibility.
  // Already named in scm-integration-service.md §3. A pre-flight check, not
  // a mutation: does CreatePullRequest have a reasonable chance of
  // succeeding for this repo+branch right now. See SOL-014.
  rpc CheckHostedReviewEligibility(CheckHostedReviewEligibilityRequest) returns (HostedReviewEligibility);
}
```

### Step 2: Append new messages

Add to the bottom of the file:

```protobuf
message CheckHostedReviewEligibilityRequest {
  string tenant_id = 1;
  ScmProvider provider = 2;
  string repo = 3;
  string head_branch = 4;
  string base_branch = 5;
}

message HostedReviewEligibility {
  bool eligible = 1;
  // ineligible_reason is set (and eligible=false) for exactly one of these,
  // in priority order — auth comes first since every other check is
  // meaningless without a usable credential:
  //   "NOT_CONNECTED"         - GetAuthStatus.connected == false
  //   "BRANCH_NOT_FOUND"      - head_branch doesn't exist on the provider yet
  //   "REVIEW_ALREADY_EXISTS" - GetPullRequestForBranch already found one
  string ineligible_reason = 2;
  // existing_pull_request is set only when ineligible_reason ==
  // "REVIEW_ALREADY_EXISTS" — lets the frontend link straight to it.
  PullRequest existing_pull_request = 3;
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
