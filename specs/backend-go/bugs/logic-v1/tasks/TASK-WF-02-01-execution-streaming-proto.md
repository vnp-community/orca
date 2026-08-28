# TASK-WF-02-01: Add `inputs_json`, `action`/`parallel` step types, and `StreamExecutionEvents` to `workflow.proto`

**From Solution:** SOL-WF-02
**Priority:** P0 — usecase/adapter tasks depend on generated stubs from this
**Service:** `workflow-service`
**File:** `backend-go/proto/orca/workflow/v1/workflow.proto`
**Depends on:** none
**Status:** `[x]` DONE — `ExecuteRequest.inputs_json`, `STEP_TYPE_ACTION`/`STEP_TYPE_PARALLEL`, `StreamExecutionEvents` RPC + `StreamExecutionEventsRequest`/`ExecutionEvent` messages added, all additive. `make proto-gen` regenerated stubs; `go build ./...` clean for `proto`, `workflow-service`, `api-gateway`. (Same `buf breaking` git-ref caveat as TASK-WF-01-03 — verified additivity by inspection.)

---

## Context

BUG-WF-02 finds variable interpolation, `action`/`parallel` step types,
and live execution streaming entirely absent. This task adds the proto
surface for all three — additive only, `buf breaking` stays clean.

## Changes to make

Extend `ExecuteRequest`:

```protobuf
message ExecuteRequest {
  string template_id = 1;
  string project_id = 2;
  string root_trace_id = 3;
  string request_id = 4;
  string inputs_json = 5; // caller-supplied {{...}} values, e.g. {"feature_description": "..."}
}
```

Add to the `StepType` enum (`workflow.proto:53-60`):

```protobuf
STEP_TYPE_ACTION = 6;   // match next unused enum ordinal
STEP_TYPE_PARALLEL = 7;
```

Add the streaming RPC and its messages to the `WorkflowService` service
block:

```protobuf
rpc StreamExecutionEvents(StreamExecutionEventsRequest) returns (stream ExecutionEvent);

message StreamExecutionEventsRequest {
  string execution_id = 1;
}
message ExecutionEvent {
  string execution_id = 1;
  string step_id = 2;       // empty for execution-level events
  string type = 3;          // step.output | step.completed | execution.completed
  string payload_json = 4;
  int64 occurred_at_unix_ms = 5;
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

Expected: clean build, `buf breaking` reports no breaking changes (only
additions).
