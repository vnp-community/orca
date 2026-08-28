# TASK-AG-02-01: Add `StopAgentSession`/`KillAgentSession` RPCs

**From Solution:** SOL-AG-02
**Priority:** P0
**Service:** `infra-fleet-service`
**File:** `backend-go/proto/orca/infrafleet/v1/infrafleet.proto`
**Depends on:** TASK-AG-01-01
**Status:** `[x]` DONE — StopAgentSession/KillAgentSession RPCs + StopAgentSessionRequest/KillAgentSessionRequest messages added to infrafleet.proto; `buf generate` + `go build ./proto/...` clean.

---

## Context

Agent-spawned PTYs live in `agent-spawner.ts`'s own `PTY_REGISTRY`, a store separate from the pty-daemon's — `pty.sendSignal`/`pty.destroy` (what `StopTerminalProcess`/`KillTerminalSession` call) cannot reach them. This adds a dedicated RPC pair that calls `agent.sendInput`/`agent.kill` instead.

## Changes to make

Add to the `InfraFleetService` service block, after `StartAgentSession`:

```protobuf
  // StopAgentSession sends agent.sendInput('\x03') — graceful interrupt,
  // BR-AG-05. Does not tear the session down; the transition to 'stopped'
  // happens once agent.exited arrives (TASK-AG-05's classifier) — status
  // transition is exit-driven, not request-driven.
  rpc StopAgentSession(StopAgentSessionRequest) returns (google.protobuf.Empty);

  // KillAgentSession sends agent.kill with the given signal (default
  // SIGKILL) — full teardown, mirrors KillTerminalSession's "mark closed
  // even if the agent call fails" discipline.
  rpc KillAgentSession(KillAgentSessionRequest) returns (google.protobuf.Empty);
```

Append messages:

```protobuf
message StopAgentSessionRequest { string session_id = 1; }
message KillAgentSessionRequest {
  string session_id = 1;
  string signal      = 2; // "SIGTERM" | "SIGKILL", default SIGKILL — mirrors agent.kill's own default
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
buf generate proto
buf breaking proto --against '.git#branch=main'
go build ./proto/...
```
