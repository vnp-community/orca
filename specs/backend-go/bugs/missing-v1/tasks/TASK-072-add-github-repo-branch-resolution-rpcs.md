# TASK-072: Add GitHub repo/branch resolution RPCs to `scmintegration.proto`

**From Solution:** SOL-012 (Design — Proto additions, shape 2)
**Priority:** P1
**Service:** `scm-integration-service`
**File:** `backend-go/proto/orca/scmintegration/v1/scmintegration.proto`
**Depends on:** TASK-071 (same file — apply after to avoid a merge conflict on the service block)
**Status:** `[x]` DONE — implemented in worktree `agent-aac2382028c6ce920` (branch `worktree-agent-aac2382028c6ce920`), **committed** as `ce750c490`. `go build`/`go vet`/`gofmt -l` clean, `buf generate`/`buf breaking` clean (additive-only). Pending merge to main + one-line RegisterRealChannels/main.go wiring.

---

## Context

`github.prForBranch` and `github.repoSlug` have no backing RPC.
`GetPullRequestForBranch` also backs `hostedReview.forBranch`'s
branch-filtered case (SOL-014, TASK-089) — it is deliberately
provider-generic (parameterized by `ScmProvider`, not GitHub-only), even
though this task only wires GitHub's adapter implementation. `ListPullRequests`
has no branch filter today; this is a distinct single-result query shape,
not a filtered list, so it's a new RPC rather than an optional field.

---

## Changes to make

**File:** `backend-go/proto/orca/scmintegration/v1/scmintegration.proto`

### Step 1: Add RPCs to the `ScmIntegrationService` service block

Find (added by TASK-071):

```protobuf
  rpc UpdateIssue(UpdateIssueRequest) returns (Issue);
}
```

Replace with:

```protobuf
  rpc UpdateIssue(UpdateIssueRequest) returns (Issue);

  // GetPullRequestForBranch — github.prForBranch AND hostedReview.forBranch's
  // branch-filtered case (SOL-014). Provider-generic: parameterized by
  // ScmProvider like every other RPC here, not a GitHub-only addition.
  rpc GetPullRequestForBranch(GetPullRequestForBranchRequest) returns (GetPullRequestForBranchResponse);

  // ResolveRepoSlug — github.repoSlug. Resolves a repo identifier (local git
  // remote URL, partial name, etc.) to the canonical "owner/name" slug.
  rpc ResolveRepoSlug(ResolveRepoSlugRequest) returns (ResolveRepoSlugResponse);
}
```

### Step 2: Append new request/response messages

Add to the bottom of the file:

```protobuf
message GetPullRequestForBranchRequest {
  string tenant_id = 1;
  ScmProvider provider = 2;
  string repo = 3;
  string head_branch = 4;
}
message GetPullRequestForBranchResponse {
  PullRequest pull_request = 1; // unset (zero-value) if no open PR for the branch
  bool found = 2;
}

message ResolveRepoSlugRequest {
  string tenant_id = 1;
  ScmProvider provider = 2;
  string candidate = 3; // remote URL, "owner/name", or bare name
}
message ResolveRepoSlugResponse {
  string owner = 1;
  string name = 2;
  string slug = 3; // "owner/name", canonical
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
