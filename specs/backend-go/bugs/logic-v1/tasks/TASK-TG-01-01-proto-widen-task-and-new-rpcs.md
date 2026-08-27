# TASK-TG-01-01: Widen `Task`/add `GetSubtree`, `RecalculateProgress`, `AddComment`, `ListComments` to `task.proto`

**From Solution:** SOL-TG-01
**Priority:** P0 — every other TG-01..TG-04 task depends on generated stubs from this
**Service:** `task-service`
**File:** `backend-go/proto/orca/task/v1/task.proto`
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

`task.proto`'s `Task` message only carries the 6 fields the current scaffold uses
(`id, tenant_id, title, status, parent_id, project_id`); the design doc's
broader field set (`description`, `complexity`/type, `assignee`, etc.) and 4
of the RPCs `task-service.md §3` already names (`GetSubtree`,
`RecalculateProgress`, `AddComment`, `ListComments`) were never added to the
generated proto (`README.md:148-157`'s own "known deviations"). This task
lands the wire contract only — usecase/domain/postgres wiring is separate
tasks.

## Changes to make

In `backend-go/proto/orca/task/v1/task.proto`, add to the `TaskService`
service block (after the existing `AIApply` line):

```protobuf
  rpc GetSubtree(GetSubtreeRequest) returns (GetSubtreeResponse);
  rpc RecalculateProgress(RecalculateProgressRequest) returns (RecalculateProgressResponse);
  rpc AddComment(AddCommentRequest) returns (AddCommentResponse);
  rpc ListComments(ListCommentsRequest) returns (ListCommentsResponse);
```

Widen `message Task` (existing 6 fields/numbers unchanged, append):

```protobuf
message Task {
  string id = 1;
  string tenant_id = 2;
  string title = 3;
  string status = 4;
  string parent_id = 5;
  string project_id = 6;

  string description = 7;
  string task_type = 8;
  string priority = 9;
  string assignee_id = 10;
  string owner_id = 11; // see SOL-TG-03's owner-intrinsic short-circuit
  google.protobuf.Timestamp due_date = 12;
  google.protobuf.DoubleValue estimated_hours = 13;
  google.protobuf.DoubleValue actual_hours = 14; // see SOL-TG-04
  string prompt_template = 15;                   // see SOL-TG-02
  string ai_context = 16;
  string ai_plan_json = 17;                      // see SOL-TG-02
  string visibility = 18;
  string worktree_id = 19;                       // see SOL-TG-04
  string agent_session_id = 20;                   // see SOL-TG-04
  int32 progress_percent = 21;
}
```

Add `import "google/protobuf/timestamp.proto";` alongside the existing
`empty.proto`/`wrappers.proto` imports at the top of the file.

Widen `CreateTaskRequest`/`UpdateTaskRequest` with the client-settable subset
(everything except `owner_id`/`agent_session_id`/`worktree_id`/
`ai_plan_json`/`progress_percent`/`actual_hours`, which are server-computed —
see SOL-TG-01's proto-additions section for the rationale):

```protobuf
message CreateTaskRequest {
  string tenant_id = 1;
  string title = 2;
  string parent_id = 3;
  string project_id = 4;
  string description = 5;
  string task_type = 6;
  string priority = 7;
  string assignee_id = 8;
  google.protobuf.Timestamp due_date = 9;
  google.protobuf.DoubleValue estimated_hours = 10;
  string prompt_template = 11;
  string ai_context = 12;
  string visibility = 13;
}

message UpdateTaskRequest {
  string id = 1;
  google.protobuf.StringValue title = 2;
  google.protobuf.StringValue status = 3;
  google.protobuf.StringValue description = 4;
  google.protobuf.StringValue task_type = 5;
  google.protobuf.StringValue priority = 6;
  google.protobuf.StringValue assignee_id = 7;
  google.protobuf.Timestamp due_date = 8;
  google.protobuf.DoubleValue estimated_hours = 9;
  google.protobuf.StringValue prompt_template = 10;
  google.protobuf.StringValue ai_context = 11;
  google.protobuf.StringValue visibility = 12;
}
```

Append new messages at the bottom of the file:

```protobuf
message GetSubtreeRequest { string root_id = 1; }
message GetSubtreeResponse {
  repeated Task tasks = 1;
  repeated AddEdgeRequest depends_on_edges = 2; // reuses the existing edge shape rather than a new message
}

message RecalculateProgressRequest { string root_id = 1; }
message RecalculateProgressResponse { int32 progress_percent = 1; }

message AddCommentRequest { string task_id = 1; string content = 2; }
message AddCommentResponse { string id = 1; string author_id = 2; string content = 3; string created_at = 4; }
message ListCommentsRequest { string task_id = 1; string page_token = 2; int32 page_size = 3; }
message ListCommentsResponse { repeated AddCommentResponse comments = 1; string next_page_token = 2; }
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

Expected: clean build; `buf breaking` reports only additions (no breaking
changes) since every new field is a new field number and every new message
is new.
