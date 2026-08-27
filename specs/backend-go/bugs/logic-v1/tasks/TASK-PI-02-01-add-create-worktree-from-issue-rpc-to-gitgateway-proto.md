# TASK-PI-02-01: Add `CreateWorktreeFromIssue` RPC to `gitgateway.proto`

**From Solution:** SOL-PI-02
**Priority:** P0 — everything else in this solution depends on generated stubs from this
**Service:** `git-gateway-service`
**File:** `backend-go/proto/orca/gitgateway/v1/gitgateway.proto`
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

BUG-PI-02 finds no `CreateWorktreeFromIssue`-shaped RPC exists anywhere.
This is a new composite saga RPC, additive to the existing
`CreateWorktree`/`CreateWorktreeRequest` (which already carries lineage
fields `parent_worktree_id`/`origin`/`capture_source`/`task_id`/
`orchestration_run_id`/`coordinator_handle`/`created_by_terminal_handle` —
see `gitgateway.proto`'s `CreateWorktreeRequest`, lines ~605-623). Note the
file's own concurrent-edit warning at the top of the service block — merge
carefully if other RPCs were added since this task was written.

## Changes to make

Add to the `GitGatewayService` service block, near `CreateWorktree`:

```protobuf
  rpc CreateWorktreeFromIssue(CreateWorktreeFromIssueRequest) returns (CreateWorktreeFromIssueResponse);
```

Add messages near `CreateWorktreeRequest`/`CreateWorktreeResponse`:

```protobuf
message ScmIssueRef {
  string provider = 1;   // "github" | "gitlab"
  string repo = 2;
  int32 number = 3;
}
message TrackerIssueRef {
  string provider = 1;   // "jira" | "linear"
  string issue_ref = 2;  // provider-native key, e.g. "ENG-123"
}

// CreateWorktreeFromIssue is the same saga as CreateWorktree with two steps
// prepended (fetch issue, derive branch name) and two appended (spawn
// agent, thread the issue link through to project-service for BR-PI-06's
// eventual status-sync consumer — see SOL-PI-03).
message CreateWorktreeFromIssueRequest {
  string project_id = 1;
  string repo_id = 2;
  string base_ref = 3;

  oneof issue_source {
    ScmIssueRef scm_issue = 4;
    TrackerIssueRef tracker_issue = 5;
  }

  bool skip_status_update = 6;   // BR-PI-06 per-call opt-out
  bool skip_agent_start = 7;
}

message CreateWorktreeFromIssueResponse {
  string worktree_id = 1;
  string path = 2;
  string head_sha = 3;
  string branch_name = 4;         // the derived name, surfaced for the UI's worktree card
  string agent_session_id = 5;    // empty if skip_agent_start or spawn failed non-fatally
  string agent_start_error = 6;   // non-fatal spawn error, if any
  bool status_update_enqueued = 7;
}
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

Expected: clean build, `buf breaking` reports no breaking changes (only additions).
