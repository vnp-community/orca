# TASK-WT-01-01: Add `name`/`path` input and `suggested_name` output to `CreateWorktree`

**From Solution:** SOL-WT-01
**Priority:** P0 — usecase/domain tasks in this set depend on these fields existing on the wire types
**Service:** `git-gateway-service`
**File:** `backend-go/proto/orca/gitgateway/v1/gitgateway.proto`
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

[SOL-WT-01](../solutions/SOL-WT-01-tao-worktree.md) closes [BUG-WT-01](../BUG-WT-01-tao-worktree-partial.md)'s gap that `docs/logic/worktree-management/BL-WT-01-tao-worktree.md`'s Input contract requires optional `name`/`path` fields that `CreateWorktreeRequest` doesn't have. This task adds them, plus a `suggested_name` output field for the [A1] duplicate-path recovery flow. Additive only — `CreateWorktreeRequest` currently uses fields 1-11 (verified in the real proto), so the two new fields go at 12/13; `CreateWorktreeResponse` currently uses 1-3, so `suggested_name` goes at 4.

## Changes to make

In `backend-go/proto/orca/gitgateway/v1/gitgateway.proto`, extend the existing messages (do not renumber any existing field):

```protobuf
message CreateWorktreeRequest {
  string project_id = 1;
  string repo_id = 2;
  string branch = 3;      // new branch name for the worktree
  string base_ref = 4;    // branch/tag/sha to branch from; typically pre-resolved via ResolvePrBase/ResolveMrBase/PrefetchCreateBase

  optional string parent_worktree_id = 5;
  optional string origin = 6;
  optional string capture_source = 7;
  optional string task_id = 8;
  optional string orchestration_run_id = 9;
  optional string coordinator_handle = 10;
  optional string created_by_terminal_handle = 11;

  // NEW (BL-WT-01 Input contract, SOL-WT-01) — empty means "derive a
  // sanitized default": name defaults to a sanitized branch name,
  // path defaults to the existing repoPath+"-"+name convention.
  optional string name = 12;
  optional string path = 13;
}
message CreateWorktreeResponse {
  string worktree_id = 1; // project-service's Worktree.id, from the saga's RecordWorktreeCreated step (TASK-193)
  string path = 2;
  string head_sha = 3;
  // NEW — populated only when the call fails with WORKTREE_PATH_EXISTS, so
  // a client that reads a non-OK response can still recover the suggestion.
  optional string suggested_name = 4;
}
```

No RPC signature change — `CreateWorktree(CreateWorktreeRequest) returns (CreateWorktreeResponse)` is unchanged, only its messages grow.

Document the 4 new error codes this saga will return (`apperrors` `Code` string, not a proto enum — matches every other usecase's failure-reporting convention in this package) as a comment above `CreateWorktreeRequest` for discoverability:

```protobuf
// CreateWorktree's usecase may fail with (via apperrors.Code, not a proto
// field): WORKTREE_NAME_INVALID (BR-WT-01 charset), WORKTREE_PATH_EXISTS
// ([A1], response still carries suggested_name), WORKTREE_BASE_REF_NOT_FOUND
// ([A2]), WORKTREE_LIMIT_EXCEEDED (BR-WT-04, cap 20/repo). See
// specs/backend-go/bugs/logic-v1/solutions/SOL-WT-01-tao-worktree.md.
```

## Verify

```bash
cd /opt/repos/orca/backend-go
buf generate proto
buf breaking proto --against '.git#branch=main'
go build ./proto/...
```

Expected: clean build; `buf breaking` reports only additions (new optional fields, no renumbering).
