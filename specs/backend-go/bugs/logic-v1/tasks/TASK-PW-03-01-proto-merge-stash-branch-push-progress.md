# TASK-PW-03-01: Add merge/stash/branch-create/soft-delete RPCs + push/pull progress streaming to `gitgateway.proto`

**From Solution:** SOL-PW-03
**Priority:** P0 — everything else in this solution depends on generated stubs from this
**Service:** `git-gateway-service`
**File:** `backend-go/proto/orca/gitgateway/v1/gitgateway.proto`
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

BUG-PW-03 identifies five missing git operations (merge, stash push/pop,
branch create, branch soft-delete) and no push/pull progress-streaming
mechanism. This task adds the wire shapes only — additive, no existing
RPC changed. `ForceDeleteBranchRequest` (`gitgateway.proto:634-637`)
already exists for `git branch -D`; `DeleteBranchRequest` below is its
`-d` sibling.

## Changes to make

In the `GitGatewayService` service block, add:

```protobuf
rpc MergeBranch(MergeBranchRequest) returns (MergeBranchResponse);
rpc StashPush(StashPushRequest) returns (StashPushResponse);
rpc StashPop(StashPopRequest) returns (StashPopResponse);
rpc CreateBranch(CreateBranchRequest) returns (CreateBranchResponse);
rpc DeleteBranch(DeleteBranchRequest) returns (DeleteBranchResponse); // soft (-d); ForceDeleteBranch remains the -D path

rpc PushStream(PushRequest) returns (stream GitProgressEvent);
rpc PullStream(PullRequest) returns (stream GitProgressEvent);
```

Add the messages (append near `ForceDeleteBranchRequest`, gitgateway.proto:634):

```protobuf
message MergeBranchRequest {
  string worktree_id = 1;
  string branch = 2;      // branch to merge INTO the current branch
  bool   no_ff = 3;       // default true when unset at the usecase layer
}
message MergeBranchResponse {
  bool success = 1;
  bool had_conflicts = 2;  // mirrors PullResponse's existing had_conflicts shape
}

message StashPushRequest {
  string worktree_id = 1;
  string message = 2;      // optional; empty = git's default stash message
  bool   include_untracked = 3;
}
message StashPushResponse { bool success = 1; }

message StashPopRequest {
  string worktree_id = 1;
  string stash_ref = 2;    // empty = pop the most recent (stash@{0})
}
message StashPopResponse {
  bool success = 1;
  bool had_conflicts = 2;
}

message CreateBranchRequest {
  string worktree_id = 1;
  string branch = 2;
  string base_ref = 3;     // empty = branch from current HEAD
  bool   checkout = 4;     // true = also switch to it
}
message CreateBranchResponse { string branch = 1; }

message DeleteBranchRequest {
  string worktree_id = 1;
  string branch = 2;
}
message DeleteBranchResponse { bool success = 1; }

// GitProgressEvent mirrors the agent's git.execStream frame shape
// (specs/agent/api/agent-rpc-catalog-git-fs.md: {type:'stream.chunk',line,source?} / {type:'stream.end',exitCode}).
message GitProgressEvent {
  string line = 1;
  string source = 2;      // "stdout" | "stderr"
  bool   is_final = 3;
  int32  exit_code = 4;   // only meaningful when is_final
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
buf generate proto
buf breaking proto --against '.git#branch=main'
go build ./proto/...
```

Expected: clean build; `buf breaking` reports only additions (5 new unary
RPCs, 2 new server-streaming RPCs, no existing RPC/message touched).
