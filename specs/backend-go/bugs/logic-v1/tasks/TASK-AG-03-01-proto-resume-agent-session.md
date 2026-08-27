# TASK-AG-03-01: Add `ResumeAgentSession` RPC

**From Solution:** SOL-AG-03
**Priority:** P0
**Service:** `infra-fleet-service`
**File:** `backend-go/proto/orca/infrafleet/v1/infrafleet.proto`
**Depends on:** TASK-AG-02-01
**Status:** `[x]` DONE — ResumeAgentSession RPC + ResumeAgentSessionRequest message added to infrafleet.proto; `buf generate` + `go build ./proto/...` clean.

---

## Context

`ResumeAgentSession` is `StartAgentSession` (TASK-AG-01) called with a non-empty `resume_id` — loads the latest `AgentSession` for a worktree, validates BR-AG-08 (7-day expiry) and BR-AG-09 (agent version match), then re-spawns.

## Changes to make

Add to the `InfraFleetService` service block, after `KillAgentSession`:

```protobuf
  // ResumeAgentSession — loads the latest AgentSession for worktree_id,
  // validates BR-AG-08 (7-day expiry) and BR-AG-09 (agent version match),
  // then re-spawns via the same path as StartAgentSession with resume_id
  // populated.
  rpc ResumeAgentSession(ResumeAgentSessionRequest) returns (AgentSession);
```

Append message:

```protobuf
message ResumeAgentSessionRequest {
  string connection_id = 1;
  string worktree_id   = 2;
  string user_id       = 3;
  string cwd           = 4;
  int32  cols          = 5;
  int32  rows          = 6;
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
buf generate proto
buf breaking proto --against '.git#branch=main'
go build ./proto/...
```
