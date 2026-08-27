# TASK-PI-04-01: Add `SubmitReview` RPC to `scmintegration.proto`

**From Solution:** SOL-PI-04
**Priority:** P0 — everything else in this solution depends on generated stubs from this
**Service:** `scm-integration-service`
**File:** `backend-go/proto/orca/scmintegration/v1/scmintegration.proto`
**Depends on:** none
**Status:** `[x] DONE — SubmitReview RPC + ReviewType/ReviewComment/SubmitReviewRequest/Review messages added to scmintegration.proto, buf generate clean.`

---

## Context

`scm-integration-service.md`'s §3 sketch already lists
`rpc SubmitReview(SubmitReviewRequest) returns (Review);` under "Reviewers &
reviews" — BUG-PI-04 confirms the implemented proto has no such RPC. This
task implements the RPC the design doc already specified.

## Changes to make

Add to the `ScmIntegrationService` service block, near
`RequestPullRequestReviewers`/`RemovePullRequestReviewers`:

```protobuf
  rpc SubmitReview(SubmitReviewRequest) returns (Review);
```

Add messages (append near `PullRequest`):

```protobuf
enum ReviewType {
  REVIEW_TYPE_UNSPECIFIED = 0;
  REVIEW_TYPE_COMMENT = 1;
  REVIEW_TYPE_APPROVE = 2;
  REVIEW_TYPE_REQUEST_CHANGES = 3;
}

message ReviewComment {
  string path = 1;       // matches Anchor.file_path (annotation.proto)
  int32 line = 2;         // matches Anchor.line (annotation.proto)
  string body = 3;        // matches Annotation.content (annotation.proto)
}

message SubmitReviewRequest {
  string tenant_id = 1;
  ScmProvider provider = 2;
  string repo = 3;
  int32 pr_number = 4;
  ReviewType review_type = 5;         // REVIEW_TYPE_UNSPECIFIED triggers BR-PI-11's default-to-REQUEST_CHANGES
  string summary_body = 6;            // top-level review comment, optional
  repeated ReviewComment comments = 7; // BR-PI-10: must be non-empty
}

message Review {
  string id = 1;
  string reviewer_id = 2;
  ReviewType state = 3;
  string submitted_at = 4;
  repeated ReviewComment comments = 5;
  string url = 6;
}
```

`ReviewComment`'s `path`/`line`/`body` field names deliberately mirror
`annotation.proto`'s `Anchor.file_path`/`Anchor.line`/`Annotation.content` —
this is what makes `api-gateway`'s composition step (TASK-PI-04-05) a pure
1:1 field copy.

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
