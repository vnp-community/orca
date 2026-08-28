# TASK-MB-03-01: Add `DispatchPrompt`/`GetQueuedPrompt` RPCs to `infrafleet.proto`

**From Solution:** SOL-MB-03
**Priority:** P0
**Service:** `infra-fleet-service`
**File:** `backend-go/proto/orca/infrafleet/v1/infrafleet.proto`
**Depends on:** none
**Status:** [x] DONE — RPCs/messages added to infrafleet.proto, `buf generate` regenerated stubs, `go build ./proto/...` passes (breaking-check skipped: no `.git` in this worktree's `backend-go/` subdir, environment-only failure).

---

## Context

`InfraFleetService` already owns terminal/PTY routing (`RouteTerminalWrite`)
— `DispatchPrompt` is a business-rule-aware sibling: it decides *whether
and when* to route a write (BR-MB-09/10/12), using the `ReadyForInput`
signal SOL-MB-02 improves. Additive-only proto change.

## Changes to make

Add to the `InfraFleetService` service block:

```protobuf
// DispatchPrompt is the ONE decision point BR-MB-09/10/12 all reduce to:
// gate on agent readiness, queue if running, require confirmation to
// overwrite an existing queued prompt.
rpc DispatchPrompt(DispatchPromptRequest) returns (DispatchPromptResponse);
rpc GetQueuedPrompt(GetQueuedPromptRequest) returns (GetQueuedPromptResponse);
```

Add messages (append to the bottom of the file):

```protobuf
message DispatchPromptRequest {
  string pty_id = 1;
  string prompt = 2;          // already decrypted by the caller (api-gateway) before this RPC
  bool   overwrite = 3;       // BR-MB-12: true only on a caller's explicit confirmation of a second dispatch
  string dispatched_by_device_id = 4; // audit/attribution — which paired mobile device sent this
}
message DispatchPromptResponse {
  enum Outcome {
    OUTCOME_UNSPECIFIED = 0;
    INJECTED_IMMEDIATELY = 1;  // BR-MB-09: agent was idle/waiting, written straight to the PTY
    QUEUED = 2;                // BR-MB-10: agent running, held for later
    REJECTED_NEEDS_CONFIRMATION = 3; // BR-MB-12: a prompt is already queued and overwrite=false
  }
  Outcome outcome = 1;
  string  existing_queued_prompt_preview = 2; // populated only for REJECTED_NEEDS_CONFIRMATION
}

message GetQueuedPromptRequest { string pty_id = 1; }
message GetQueuedPromptResponse {
  bool   has_queued_prompt = 1;
  string prompt = 2;
  int64  queued_at_unix_ms = 3;
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
