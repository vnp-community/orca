# BE-SOL-006: `infra-fleet-service` streaming RPC for agent spawn output/exit

**Resolves:** [CR-TG-006](../../../../../../docs/crs/v4/task-graph/CR-TG-006-task-execute-streaming-relay.md) (backend-go portion — the `agent/`-side `chunk` notification is [SOL-AG-TG-002](../../../../../agent/crs/v4/task-graph/solutions/SOL-AG-TG-002-agent-chunk-streaming.md))
**Service:** `infra-fleet-service` only
**Depends on:** [BE-SOL-004](./BE-SOL-004-orchestration-service-coordinator-run-lifecycle.md)/[BE-SOL-005](./BE-SOL-005-task-agent-execution-permission-and-complex-executor.md) (need a real dispatch path to stream from)
**Affected files (proposed):**
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto` (new streaming RPC)
- `backend-go/services/infra-fleet-service/internal/adapter/grpc/server.go` (handler)
- `backend-go/services/infra-fleet-service/internal/usecase/` (subscribe-and-forward logic)
**Status:** 📋 Proposed — not yet implemented

---

## Design rationale (grounded in real code)

`infrafleet.proto`'s real RPC surface already has the exact precedent this
solution follows: `AttachPty(stream PtyClientFrame) returns (stream
PtyServerFrame)` (`infrafleet.proto:116`) and `AttachScreencast` (`:129`) are
both real, working bidirectional-streaming RPCs that subscribe to a
Dev-Server-Agent-side session and forward its out-of-band notifications to
a gRPC stream. `Relay`/`RelayByDevServer` (`:47,54`) — the unary RPC
`SimpleExecutor`/workflow-service's step executors use today — cannot
receive `agent.spawn`'s `agent.output`/`agent.exited` notifications by
construction (a unary response completes once, notifications arrive
indefinitely after). This solution adds one more entry in the same
streaming family, not a new mechanism.

## Design — `AttachAgentSpawn`

```protobuf
// infrafleet.proto
rpc AttachAgentSpawn(AttachAgentSpawnRequest) returns (stream AgentSpawnEvent);

message AttachAgentSpawnRequest {
  string connection_id = 1;
  string spawn_id = 2; // = agent.spawn's ptyId — same identifier space AttachPty already keys on
}
message AgentSpawnEvent {
  oneof event {
    AgentOutputChunk output = 1; // mirrors agent.output {ptyId, data} verbatim — no new wire shape on the agent side
    AgentExitedEvent exited = 2; // mirrors agent.exited {ptyId, exitCode} verbatim
  }
}
```

Server-streaming only (client → server: one request; server → client: many
events) rather than `AttachPty`'s bidirectional shape — a spawned agent
process's output/exit is inherently one-directional from the caller's
point of view (no equivalent of `PtyClientFrame`'s "send keystrokes" need
for this use case). **Verify before implementing:** confirm
`infra-fleet-service`'s existing relay-connection registry (whatever
`AttachPty`'s handler subscribes to internally) exposes a way to subscribe
to `agent.output`/`agent.exited` notifications keyed by `ptyId`/`spawn_id`
for a given `connection_id` — this solution assumes that registry already
exists (since `AttachPty` proves the general mechanism works) but the exact
internal subscription API needs a direct read of `AttachPty`'s handler
before coding this RPC's implementation.

## Not in scope (per the CR)

- Any `agent/` protocol change — `agent.output`/`agent.exited` shapes are
  reused verbatim, per the CR's own design.
- `chunk` notifications for `agent.execPrompt`/`shell.exec` (one-shot exec,
  not `agent.spawn`) — that's [SOL-AG-TG-002](../../../../../agent/crs/v4/task-graph/solutions/SOL-AG-TG-002-agent-chunk-streaming.md),
  a different RPC family (one-shot request/response vs. long-lived spawn).
- `orchestration-service` actually calling this RPC as part of dispatch —
  owned by whoever implements [BE-SOL-004](./BE-SOL-004-orchestration-service-coordinator-run-lifecycle.md)'s
  `AdvancePendingRuns` dispatch step.

## Test plan

- `AttachAgentSpawn` on a real `agent.spawn` session receives output frames
  matching what was printed, then exactly one `exited` event with the right
  exit code.
- Disconnecting the gRPC stream mid-session does not crash
  `infra-fleet-service` or leak the underlying subscription (test: attach,
  disconnect, attach again on the same `spawn_id`, confirm no duplicate/stale
  event delivery).

## References

- [CR-TG-006](../../../../../../docs/crs/v4/task-graph/CR-TG-006-task-execute-streaming-relay.md)
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto:116,129` (`AttachPty`/`AttachScreencast` precedent)
