# TASK-TG-006-01: `AttachAgentSpawn` server-streaming RPC (`infra-fleet-service`)

**From Solution:** BE-SOL-006
**Priority:** P2
**Service:** `infra-fleet-service`
**File:** `backend-go/proto/orca/infrafleet/v1/infrafleet.proto` (new streaming RPC), `backend-go/services/infra-fleet-service/internal/adapter/grpc/server.go` (handler), `backend-go/services/infra-fleet-service/internal/usecase/` (new subscribe-and-forward usecase)
**Depends on:** TASK-TG-004-03/TASK-TG-005-02 (need a real dispatch path to stream from — this RPC has no callers until orchestration-service's `AdvancePendingRuns` actually dispatches something via `agent.spawn`)
**Status:** `[ ]` TODO

---

## Context

**"Verify before implementing" from BE-SOL-006, now partially done —
finish the check before coding.** BE-SOL-006 itself says: *"confirm
`infra-fleet-service`'s existing relay-connection registry (whatever
`AttachPty`'s handler subscribes to internally) exposes a way to subscribe
to `agent.output`/`agent.exited` notifications keyed by `ptyId`/`spawn_id`
for a given `connection_id`."* Direct read confirms the mechanism exists and
where: `AttachPty`'s gRPC handler
(`internal/adapter/grpc/server.go:917-951`, read in full) delegates to
`s.attachPty.Execute(ctx, inbound)` (a usecase, not inline logic), which
returns `(outbound <-chan ..., errCh <-chan error)` — a channel-based
pump the handler forwards to `stream.Send`. The actual subscription
registry this pumps from lives in
`internal/adapter/devserveragent/client.go` (confirmed: `sessions
map[string]*session` at line 95, and a documented
"Subscribes BEFORE issuing the call, same race-avoidance [pattern]" note at
line 469; `session_test.go`'s `TestSession_RouteNotification_*` tests name
the actual routing method, `RouteNotification`). **Read
`internal/adapter/devserveragent/client.go` and `session.go` (or wherever
`RouteNotification` is defined — confirm the exact filename before coding)
in full before writing this task's usecase** — the precise API to subscribe
by `ptyId`-equivalent (here, `spawn_id`) for `agent.output`/`agent.exited`
specifically (as opposed to whatever `AttachPty` subscribes to) needs to be
confirmed against that file's real method signatures, not assumed from
`AttachPty`'s existence alone.

`infrafleet.proto`'s real precedent RPCs are confirmed at exactly the lines
BE-SOL-006 cites: `AttachPty(stream PtyClientFrame) returns (stream
PtyServerFrame)` at `infrafleet.proto:116`, `AttachScreencast` at `:129` —
both real, working bidirectional-streaming RPCs. `Relay`/`RelayByDevServer`
(confirmed at lines 47/54, not 47/54 exactly as originally guessed but
matching — direct read shows `rpc Relay(...)` at line 47 and `rpc
RelayByDevServer(...)` at line 54) are the unary RPCs
`SimpleExecutor`/workflow-service's step executors use today, and cannot
receive `agent.spawn`'s indefinite `agent.output`/`agent.exited`
notifications by construction (a unary response completes once).

## Changes to make

**1. `infrafleet.proto`** — server-streaming (not bidirectional like
`AttachPty` — a spawned agent process's output/exit is one-directional from
the caller's point of view, no "send keystrokes" need):

```protobuf
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
message AgentOutputChunk { string data = 1; }
message AgentExitedEvent { int32 exit_code = 1; }
```

**2. `internal/usecase/attach_agent_spawn.go`** (new) — follow
`AttachPty`'s usecase shape exactly (find its file — likely
`attach_pty.go` given the test file `attach_pty_test.go` referenced above —
and mirror its `Execute(ctx, inbound) (outbound, errCh)`-or-equivalent
signature, adapted for server-streaming's simpler one-directional shape:
this usecase likely needs no `inbound` channel at all, only a `spawnID`
input and an `outbound` channel of events):

```go
type AttachAgentSpawn struct {
	sessions SpawnSessionRegistry // the real subscription API confirmed by reading devserveragent/client.go — name TBD until that read happens
}

func (uc *AttachAgentSpawn) Execute(ctx context.Context, connectionID, spawnID string) (<-chan AgentSpawnMessage, <-chan error) {
	// Subscribe to spawnID's agent.output/agent.exited notifications on
	// connectionID's session, same race-avoidance ordering as AttachPty's
	// "subscribe BEFORE issuing the call" precedent
	// (devserveragent/client.go:469's doc comment).
}
```

**3. `internal/adapter/grpc/server.go`** — handler mirrors `AttachPty`'s
structure (`server.go:917-951`) minus the inbound pump (server-streaming
has no client-to-server frames after the initial request):

```go
func (s *Server) AttachAgentSpawn(req *infrafleetv1.AttachAgentSpawnRequest, stream infrafleetv1.InfraFleetService_AttachAgentSpawnServer) error {
	ctx := withTenantFromStreamMetadata(stream.Context()) // same tenant-extraction workaround AttachPty/AttachScreencast both need
	outbound, errCh := s.attachAgentSpawn.Execute(ctx, req.GetConnectionId(), req.GetSpawnId())
	for {
		select {
		case msg, ok := <-outbound:
			if !ok {
				outbound = nil
				continue
			}
			if err := stream.Send(toProtoAgentSpawnEvent(msg)); err != nil {
				return err
			}
		case err, ok := <-errCh:
			if !ok {
				return nil
			}
			if err != nil {
				return apperrors.ToGRPCStatus(err)
			}
			return nil
		}
	}
}
```

## Not in scope (per BE-SOL-006)

- Any `agent/` protocol change — `agent.output`/`agent.exited` shapes
  reused verbatim.
- `chunk` notifications for `agent.execPrompt`/`shell.exec` (one-shot exec)
  — that's [SOL-AG-TG-002](../../../../../agent/crs/v4/task-graph/solutions/SOL-AG-TG-002-agent-chunk-streaming.md),
  a different RPC family.
- `orchestration-service` actually calling this RPC as part of dispatch —
  owned by whoever implements TASK-TG-004-03's deferred dispatch step.

## Test plan

- `AttachAgentSpawn` on a real `agent.spawn` session receives output frames
  matching what was printed, then exactly one `exited` event with the right
  exit code.
- Disconnecting the gRPC stream mid-session does not crash
  `infra-fleet-service` or leak the underlying subscription — test: attach,
  disconnect, attach again on the same `spawn_id`, confirm no duplicate/
  stale event delivery (per BE-SOL-006's own test plan, this is the
  regression proof that the subscription is torn down cleanly on stream
  disconnect, mirroring however `AttachPty`'s existing tests prove the same
  for PTY sessions).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/usecase/... -run TestAttachAgentSpawn -v
go test ./services/infra-fleet-service/internal/adapter/grpc/... -v
```

Expected: clean build; output/exit event ordering test passes; the
disconnect-then-reattach test confirms no stale subscription persists past
a stream's lifetime (mirror whatever assertion `AttachPty`'s own equivalent
test uses, if one exists — check `attach_pty_test.go` for the precedent
before writing this one from scratch).
