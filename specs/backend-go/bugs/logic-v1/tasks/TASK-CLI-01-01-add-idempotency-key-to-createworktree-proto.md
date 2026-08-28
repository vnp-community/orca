# TASK-CLI-01-01: Add `idempotency_key` to `CreateWorktreeRequest`

**From Solution:** SOL-CLI-01
**Priority:** P0 — every other task in this set builds on this field existing on the wire
**Service:** `git-gateway-service` (proto)
**File:** `backend-go/proto/orca/gitgateway/v1/gitgateway.proto`
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

BR-CLI-01 needs `orca worktree create` retries (CI re-runs the same command) to return the original worktree instead of re-running `git worktree add`. `CreateWorktreeRequest` currently has no field to carry a caller-supplied dedupe key. This task adds it — additive only, so `buf breaking` stays clean.

## Changes to make

In `backend-go/proto/orca/gitgateway/v1/gitgateway.proto`, extend `CreateWorktreeRequest` (currently fields 1-11):

```protobuf
message CreateWorktreeRequest {
  string project_id = 1;
  string repo_id = 2;
  string branch = 3;      // new branch name for the worktree
  string base_ref = 4;    // branch/tag/sha to branch from; typically pre-resolved via ResolvePrBase/ResolveMrBase/PrefetchCreateBase

  // Optional lineage-capture context, forwarded as-is to project-service's
  // RecordWorktreeCreated by the CreateWorktree saga — see
  // project.proto's WorktreeLineageEntry doc comment for the shape this
  // backs. Explicit-capture only; caller (wscompat's worktree.create) never
  // needs to set capture_confidence.
  optional string parent_worktree_id = 5;
  optional string origin = 6;
  optional string capture_source = 7;
  optional string task_id = 8;
  optional string orchestration_run_id = 9;
  optional string coordinator_handle = 10;
  optional string created_by_terminal_handle = 11;

  // idempotency_key: caller-supplied dedupe key (BR-CLI-01). orca-cli
  // defaults to sha256(project_id|repo_id|branch) when the user passes
  // none. A second CreateWorktree call with the same (project_id,
  // idempotency_key) returns the existing worktree instead of re-running
  // `git worktree add`. Empty means "no dedupe requested" — every existing
  // caller (wscompat's worktree.create) keeps working unchanged.
  optional string idempotency_key = 12;
}
```

`CreateWorktreeResponse` is unchanged — the idempotent-replay path returns the same `{worktree_id, path, head_sha}` shape a fresh create would.

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
grep -n "IdempotencyKey" proto/gen/go/orca/gitgateway/v1/gitgateway.pb.go
```

Expected: clean build, `buf breaking` reports no breaking changes (only additions), and the generated Go struct has an `IdempotencyKey *string` field.
