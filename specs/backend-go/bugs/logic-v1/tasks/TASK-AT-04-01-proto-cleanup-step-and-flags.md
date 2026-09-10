# TASK-AT-04-01: Proto changes — `STEP_TYPE_CLEANUP_WORKTREES`, `allow_open_pr`, `Worktree.status`

**From Solution:** SOL-AT-04
**Priority:** P0 — everything else in this solution depends on generated stubs from this
**Service:** `workflow-service` / `git-gateway-service` / `project-service`
**File:** `backend-go/proto/orca/workflow/v1/workflow.proto`, `backend-go/proto/orca/gitgateway/v1/gitgateway.proto`, `backend-go/proto/orca/project/v1/project.proto`
**Depends on:** none
**Status:** `[x]` DONE — STEP_TYPE_CLEANUP_WORKTREES added to workflow.proto; allow_open_pr added to RemoveWorktreeRequest (gitgateway.proto); status/ListWorktreesRequest filters added to project.proto; buf generate + go build clean.

---

## Context

Three separate proto surfaces need additive changes before any usecase code
in this solution can compile: a new `StepType` enum value, a new
`RemoveWorktreeRequest` safety-override flag, and a `Worktree.status` +
`ListWorktreesRequest` filters.

## Changes to make

`workflow.proto` — add to the `StepType` enum (use the next free number
after the existing 5 values):

```protobuf
enum StepType {
  // ... existing 5 values ...
  STEP_TYPE_CLEANUP_WORKTREES = 6; // NEW — BL-AT-04
}
```

`gitgateway.proto` — add to `RemoveWorktreeRequest`:

```protobuf
message RemoveWorktreeRequest {
  string worktree_id = 1;
  bool   force = 2;          // unchanged meaning: override the uncommitted-changes check (BR-AT-11)
  bool   allow_open_pr = 3;  // NEW — separate, explicit override for BR-AT-12; NEVER set true by the cleanup_worktrees path
}
```

(Check the actual current field numbers in `RemoveWorktreeRequest` before
picking `= 3`.)

`project.proto` — add to `Worktree` and a new request message:

```protobuf
message Worktree {
  // ... existing fields ...
  string status = N; // NEW — "active" | "completed" | "error" | "stopped"; orthogonal to activation_state
}

message ListWorktreesRequest {
  string project_id = 1;
  repeated string status_in = 2;             // NEW — optional; unset = no filtering
  google.protobuf.Timestamp older_than = 3;  // NEW — optional; created_at cutoff
}
```

(Check whether `ListWorktreesRequest` already exists with a different field
1 — extend the existing message rather than redefining it.)

## Verify

```bash
cd /opt/repos/orca/backend-go
buf generate proto
buf breaking proto --against '.git#branch=main'
go build ./proto/...
```

Expected: clean build; `buf breaking` reports only additions.
