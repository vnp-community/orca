# TASK-192: Add `CreateWorktree`/`RemoveWorktree`/`ForceDeleteBranch`/`DetectWorktrees`/`PrefetchCreateBase`/`ResolvePrBase`/`ResolveMrBase` RPCs to `gitgateway.proto`

**From Solution:** SOL-031 (design part 1: "the new git-gateway-service RPCs")
**Priority:** P0 — everything else in this solution depends on generated stubs from this
**Service:** `git-gateway-service`
**File:** `backend-go/proto/orca/gitgateway/v1/gitgateway.proto`
**Depends on:** none
**Status:** `[x]` DONE — implemented in worktree `agent-aa8bd8599a599323a` (team/terminal/workflow/worktree pass, merged into `integration/missing-v1` as commit `baa34819a`); this task doc's own Status line was never updated by that implementing pass (a task-doc-capture gap, not a missing-code gap) — verified against the current merged code+tests during a later re-audit: build/vet/test clean.

---

## Context

BUG-031's key finding: `project-service` can answer "what worktrees exist"
and toggle activation/rename bookkeeping (`ListWorktrees`/
`SetWorktreeActivation`/`RenameWorktree` — all real, `project.proto:36-38`),
but **nothing in backend-go can actually create or remove an on-disk git
worktree** — `gitgateway.proto` has zero worktree RPCs. This task adds the
proto surface only; the saga/compensation logic coordinating with
`project-service`'s `RecordWorktreeCreated`/`RecordWorktreeRemoved` is
TASK-193, and the required (not optional) `ForceDeleteBranch` interface fix
is TASK-194.

`git-gateway-service` is authoritative for on-disk existence;
`project-service` stays authoritative for bookkeeping metadata
(`project-service.md` §4: "Never authoritative for whether the worktree
still exists on disk"). `DetectWorktrees` gives that read-side
reconciliation a concrete RPC instead of leaving it unspecified — see the
"raw paths, not a diff" note below.

## Changes to make

**File:** `backend-go/proto/orca/gitgateway/v1/gitgateway.proto`

Add to the `GitGatewayService` block, after `GenerateCommitMessage`:

```protobuf
  rpc GenerateCommitMessage(GenerateCommitMessageRequest) returns (GenerateCommitMessageResponse);

  rpc CreateWorktree(CreateWorktreeRequest) returns (CreateWorktreeResponse);
  rpc RemoveWorktree(RemoveWorktreeRequest) returns (google.protobuf.Empty);
  // Required on every GitExecutor implementation from day one — TASK-194
  // makes it a required interface method, not an optional one, closing the
  // old TS backend's crash-bug class (forceDeletePreservedBranch? was
  // optional and only one provider implemented it).
  rpc ForceDeleteBranch(ForceDeleteBranchRequest) returns (google.protobuf.Empty);

  // DetectWorktrees is the concrete form of project-service.md §4's
  // "git-gateway-service reconciles on demand" — returns raw on-disk
  // paths, no bookkeeping join (that diff happens at api-gateway's edge
  // layer, TASK-195, per 05-data-architecture.md's cross-service
  // aggregation rule).
  rpc DetectWorktrees(DetectWorktreesRequest) returns (DetectWorktreesResponse);

  rpc PrefetchCreateBase(PrefetchCreateBaseRequest) returns (PrefetchCreateBaseResponse);
  rpc ResolvePrBase(ResolvePrBaseRequest) returns (ResolveBaseResponse);
  rpc ResolveMrBase(ResolveMrBaseRequest) returns (ResolveBaseResponse);
}
```

Add `import "google/protobuf/empty.proto";` at the top of the file if not
already present (`gitgateway.proto` currently has no `Empty`-typed
response, so check before assuming the import already exists).

Add the new messages at the bottom of the file:

```protobuf
message CreateWorktreeRequest {
  string project_id = 1;
  string repo_id = 2;
  string branch = 3;      // new branch name for the worktree
  string base_ref = 4;    // branch/tag/sha to branch from; typically pre-resolved via ResolvePrBase/ResolveMrBase/PrefetchCreateBase
}
message CreateWorktreeResponse {
  string worktree_id = 1; // project-service's Worktree.id, from the saga's RecordWorktreeCreated step (TASK-193)
  string path = 2;
  string head_sha = 3;
}

message RemoveWorktreeRequest {
  string worktree_id = 1;
  bool   force = 2;       // maps to `git worktree remove --force` (uncommitted changes present)
}

message ForceDeleteBranchRequest {
  string worktree_id = 1;
  string branch = 2;
}

message DetectWorktreesRequest  { string repo_id = 1; }
message DetectWorktreesResponse {
  repeated string on_disk_paths = 1; // raw `git worktree list --porcelain` result for the repo
}

message PrefetchCreateBaseRequest  { string repo_id = 1; string base_ref = 2; }
message PrefetchCreateBaseResponse { string resolved_sha = 1; } // ensures base_ref is fetched/up to date

message ResolvePrBaseRequest { string repo_id = 1; int32 pr_number = 2; }
message ResolveMrBaseRequest { string repo_id = 1; int32 mr_number = 2; }
message ResolveBaseResponse  { string base_branch = 1; string base_sha = 2; }
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

Expected: clean build, `buf breaking` reports only additions (7 new RPCs,
9 new messages, 1 new import).
