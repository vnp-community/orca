# TASK-AG-04-01: Add `SwitchAgentAccount` RPC

**From Solution:** SOL-AG-04
**Priority:** P0
**Service:** `infra-fleet-service`
**File:** `backend-go/proto/orca/infrafleet/v1/infrafleet.proto`
**Depends on:** TASK-AG-03-01
**Status:** `[x]` DONE — SwitchAgentAccount RPC + SwitchAgentAccountRequest message added to infrafleet.proto; `buf generate` + `go build ./proto/...` clean.

---

## Context

`SwitchAgentAccount` is a saga composing `KillAgentSession` (TASK-AG-02), `ai-provider-service.ResolveProvider`, and `StartAgentSession`/`ResumeAgentSession` (TASK-AG-01/03) — this task only adds its proto surface.

## Changes to make

Add to the `InfraFleetService` service block, after `ResumeAgentSession`:

```protobuf
  rpc SwitchAgentAccount(SwitchAgentAccountRequest) returns (AgentSession);
```

Append message:

```protobuf
message SwitchAgentAccountRequest {
  string connection_id = 1;
  string worktree_id   = 2;
  string user_id       = 3;
  string project_id    = 4; // threaded into ResolveProvider's cascade exclusion
  string cwd           = 5;
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
buf generate proto
buf breaking proto --against '.git#branch=main'
go build ./proto/...
```
