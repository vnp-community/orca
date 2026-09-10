# TASK-PW-04-01: `task.proto` — `task_number`, `worktree_id`, `pr_url`, `FindTaskByNumber`

**From Solution:** SOL-PW-04
**Priority:** P0 — everything else touching `task.proto`'s shape depends on generated stubs from this
**Service:** `task-service`
**File:** `backend-go/proto/orca/task/v1/task.proto`
**Depends on:** none
**Status:** `[x]` DONE — added Task.task_number/worktree_id/pr_url, UpdateTaskRequest.pr_url/worktree_id, FindTaskByNumber RPC + messages; buf generate clean, no breaking changes vs origin/main

---

## Context

Flow 2 (commit message `#TG-123` closes a task) and Flow 4 (task↔worktree
linkage for later attribution) both need wire fields `Task` doesn't have
yet. `Task` currently has `id/tenant_id/title/status/parent_id/project_id`
(`task.proto:49-56`); `UpdateTaskRequest` currently has only
`id/title/status` (`task.proto:156-160`). This task is additive-only.

## Changes to make

In `task.proto`, extend `Task`:

```protobuf
message Task {
  string id = 1;
  string tenant_id = 2;
  string title = 3;
  string status = 4;
  string parent_id = 5;
  string project_id = 6; // added for Epic C's HasActiveExecutions
  // Added SOL-PW-04: a per-project sequential number so a commit message
  // can reference "#TG-42" without embedding a UUID. Assigned once at
  // CreateTask, immutable, backed by a per-project sequence — never
  // reused even if the task is deleted, matching GitHub/Jira issue-number
  // semantics.
  int64 task_number = 7;
  // Mirrors project-service's Worktree.TaskID (project.proto:286) from
  // the task side — task-service cannot join project-service's table
  // directly (no service reads another service's tables), so this is
  // written back explicitly by whichever saga first associates the two.
  // Empty until an agent run or commit-close saga sets it.
  string worktree_id = 8;
  // Set by the PR-creation write-back saga — empty until a PR
  // referencing this task's #TG-N is created.
  string pr_url = 9;
}
```

Extend `UpdateTaskRequest`:

```protobuf
message UpdateTaskRequest {
  string id = 1;
  google.protobuf.StringValue title = 2;
  google.protobuf.StringValue status = 3;
  google.protobuf.StringValue pr_url = 4;
  google.protobuf.StringValue worktree_id = 5;
}
```

Add the read RPC the commit-message saga needs:

```protobuf
// FindTaskByNumber resolves a project-scoped "#TG-N" reference to a task
// id. Project-scoped, not tenant-wide: two different projects can each
// have their own #TG-42, matching how GitHub/Jira issue numbers are
// repo/project-scoped, not org-wide.
rpc FindTaskByNumber(FindTaskByNumberRequest) returns (FindTaskByNumberResponse);
message FindTaskByNumberRequest {
  string project_id = 1;
  int64 task_number = 2;
}
message FindTaskByNumberResponse {
  Task task = 1; // empty/NOT_FOUND if no task in project_id has this number
}
```

Add `rpc FindTaskByNumber(...)` to the `TaskService` service block.

## Verify

```bash
cd /opt/repos/orca/backend-go
buf generate proto
buf breaking proto --against '.git#branch=main'
go build ./proto/...
```

Expected: clean build; `buf breaking` reports only additions (3 new
`Task` fields, 2 new `UpdateTaskRequest` fields, 1 new RPC + 2 new
messages).
