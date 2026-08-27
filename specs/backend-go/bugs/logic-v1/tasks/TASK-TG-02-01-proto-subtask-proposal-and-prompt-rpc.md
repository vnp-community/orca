# TASK-TG-02-01: Proto — widen `SubtaskProposal`, add `raw_response`, add `GenerateAgentPrompt` RPC

**From Solution:** SOL-TG-02
**Priority:** P1
**Service:** `task-service`
**File:** `backend-go/proto/orca/task/v1/task.proto`
**Depends on:** TASK-TG-01-01 (both touch `task.proto` — land this after to avoid conflicting field-number allocation)
**Status:** `[ ]` TODO

---

## Context

`SubtaskProposal` currently only has `title`/`description` — the AI-relay
response needs `type`/`estimated_hours`/`depends_on`/`prompt_template` to
carry a structured breakdown. `AIDecomposeResponse` needs a `raw_response`
field so a JSON-parse failure can still show the caller what the AI
actually returned. `GenerateAgentPrompt` is an entirely new RPC — the
"second AI flow" this solution adds.

## Changes to make

In `backend-go/proto/orca/task/v1/task.proto`, replace the existing
`SubtaskProposal` message:

```protobuf
message SubtaskProposal {
  string title = 1;
  string description = 2;
  string type = 3; // task|bug|feature — mirrors Task.task_type
  google.protobuf.DoubleValue estimated_hours = 4;
  // DependsOnIndices names OTHER proposals in the SAME AIDecompose response
  // by their 0-based position — proposals have no Task.ID until AIApply
  // creates them.
  repeated int32 depends_on_indices = 5;
  string prompt_template = 6;
}
```

Widen `AIDecomposeResponse` and `AIApplyRequest`:

```protobuf
message AIDecomposeResponse {
  repeated SubtaskProposal proposals = 1;
  string raw_response = 2; // unparsed AI JSON — persisted to ai_plan_json, shown on parse failure
  string notes = 3;
}

message AIApplyRequest {
  string task_id = 1;
  repeated SubtaskProposal proposals = 2;
  string raw_ai_response = 3; // echoed back from AIDecomposeResponse.raw_response
}
```

Add the new RPC to the `TaskService` service block:

```protobuf
  rpc GenerateAgentPrompt(GenerateAgentPromptRequest) returns (GenerateAgentPromptResponse);
```

Append new messages:

```protobuf
message GenerateAgentPromptRequest { string task_id = 1; bool save = 2; }
message GenerateAgentPromptResponse { string prompt = 1; }
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

Expected: clean build. `buf breaking` may flag `SubtaskProposal`'s field
reuse as safe (field numbers 1/2 unchanged, 3-6 are new) — confirm no
existing field was renumbered or removed; `AIDecompose`/`AIApply` have no
live wscompat/REST callers yet (per BUG-034), so widening
`SubtaskProposal`'s shape is not a breaking wire-format change for any real
consumer.
