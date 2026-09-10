# SOL-029: Add a PTY surface to `infra-fleet-service` — `orca.infrafleet.v1`'s `AttachPty` stream plus 8 lifecycle RPCs

**Resolves:** [BUG-029](../BUG-029-terminal-channels-not-implemented.md)
**Service:** `infra-fleet-service` (new proto surface + adapter/usecase work) + `api-gateway` (10 new `wscompat` channels, `AttachPty` push-piped per SOL-035's pattern)
**Affected files (proposed):**
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto`
- `backend-go/services/infra-fleet-service/internal/domain/terminal_session.go` (new)
- `backend-go/services/infra-fleet-service/internal/usecase/spawn_terminal_session.go`, `attach_pty.go`, `resize_terminal_session.go`, `kill_terminal_session.go`, `stop_terminal_process.go`, `wait_terminal_session.go`, `list_terminal_sessions.go`, `focus_terminal_session.go`, `get_terminal_agent_status.go`, `inspect_terminal_process.go`, `ports.go` (all new)
- `backend-go/services/infra-fleet-service/internal/adapter/postgres/terminal_session_repository.go` (new)
- `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/client.go`, `methods.go` (extended: `Stream`, `pty.*` typed wrappers)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`, `push_bridge.go`
**Status:** ✅ Implemented — all 8 task(s) (TASK-180–187) DONE; see each task file's own Status/Verify section for evidence.

---

## Why this is the highest-value proposal in this batch

BUG-029 confirmed **zero** PTY-shaped RPC exists anywhere in backend-go's
proto surface — `infrafleet.proto` has only registry/connection/relay
RPCs, no `terminal.*` equivalent, and all 10 frontend `terminal.*` methods
dead-end at `notImplementedHandler`. Every other `terminal.*` consumer in
the frontend (`launch-agent-background-session.ts`,
`active-agent-note-send.ts`, `runtime-terminal-inspection.ts`, …) is
blocked on this. Unlike SOL-028/SOL-030, this is not a thin gap on top of
an existing design — `infra-fleet-service.md` §3 sketches 5 of the needed
RPCs (`SpawnTerminalSession`/`RouteTerminalWrite`/`ResizeTerminalSession`/
`KillTerminalSession`/`ListTerminalSessions`) but its own §7 dependency
section describes a 6th piece — a dedicated PTY-data **streaming** RPC —
that §3's list never actually names. This solution closes that
self-inconsistency and adds the remaining lifecycle RPCs BUG-029's 10
methods need beyond what §3 sketched.

---

## Reconciling `infra-fleet-service.md` §3's RPC list against its own §7

§3 (`infra-fleet-service.md:123-132`) sketches terminal RPCs but explicitly
notes "no PTY bytes cross this API" for them — they're control-plane only.
§7 (`infra-fleet-service.md:360-366`), describing how `api-gateway` uses
this service, says something §3's list has no RPC for:

> `api-gateway` ... calls `SpawnTerminalSession`/resolves the connection,
> then opens the actual PTY I/O stream directly against **this service's
> gRPC server-streaming terminal-data endpoint** (bytes do not round-trip
> through `ResolveConnection` per-keystroke — only the initial
> spawn/resize/kill control calls are unary RPCs; **the data stream is a
> dedicated server-streaming RPC** once the route is resolved).

There is no RPC named for this "server-streaming terminal-data endpoint"
in §3. This solution adds it — as a **bidirectional** streaming RPC, not
server-streaming-only, because §7's own framing has a second gap: it names
`RouteTerminalWrite` (§3) as the thing that "routes" writes, but that
RPC's own doc comment (`infra-fleet-service.md:126-129`) says it "only
routes control-plane metadata (which connection a `ptyId` belongs to)" and
is explicitly **not** how per-byte input travels. Nothing in §3 is
actually a client→server data channel, then — server-streaming alone can't
carry `terminal.send`'s input bytes back to the PTY. A single bidirectional
streaming RPC resolves both gaps at once and is the shape
`08-inter-service-communication.md`'s Option B sketch already anticipates
for PTY I/O generally ("bidirectional streaming for PTY I/O" — that
document's own words, describing the *agent* protocol's hypothetical
future gRPC shape; this solution applies the same shape one layer up, at
the `infra-fleet-service` boundary, while leaving the actual agent
protocol on Option A, unchanged — see below).

---

## Design — Proto additions (`infrafleet.proto`, package `orca.infrafleet.v1`)

```protobuf
service InfraFleetService {
  // ... existing RPCs unchanged ...

  // --- Terminal/PTY lifecycle (control-plane, unary) ---
  rpc SpawnTerminalSession(SpawnTerminalSessionRequest) returns (SpawnTerminalSessionResponse);
  rpc ResizeTerminalSession(ResizeTerminalSessionRequest) returns (google.protobuf.Empty);
  rpc KillTerminalSession(KillTerminalSessionRequest) returns (google.protobuf.Empty);      // terminal.close
  rpc StopTerminalProcess(StopTerminalProcessRequest) returns (google.protobuf.Empty);      // terminal.stop — interrupt, not teardown
  rpc ListTerminalSessions(ListTerminalSessionsRequest) returns (ListTerminalSessionsResponse);
  rpc WaitTerminalSession(WaitTerminalSessionRequest) returns (WaitTerminalSessionResponse); // terminal.wait — bounded blocking poll
  rpc FocusTerminalSession(FocusTerminalSessionRequest) returns (google.protobuf.Empty);     // terminal.focus — bookkeeping touch
  rpc GetTerminalAgentStatus(GetTerminalAgentStatusRequest) returns (GetTerminalAgentStatusResponse); // backs BOTH agentStatus and isRunningAgent
  rpc InspectTerminalProcess(InspectTerminalProcessRequest) returns (InspectTerminalProcessResponse); // best-effort, see below

  // --- Terminal/PTY I/O (the "server-streaming terminal-data endpoint"
  // infra-fleet-service.md §7 names but §3 never enumerates — see above.
  // Bidirectional: client frames carry input+resize, server frames carry
  // output+exit. This is the RPC api-gateway's wscompat bridge (SOL-035)
  // opens once per terminal.create and pipes into `push` frames. ---
  rpc AttachPty(stream PtyClientFrame) returns (stream PtyServerFrame);
}

message SpawnTerminalSessionRequest {
  string connection_id = 1;  // empty = host-local (rare in server deployment, per BUG-029's "no local shell" finding — see below)
  string cwd = 2;
  string shell = 3;          // optional; agent applies its own default if empty
  int32  cols = 4;
  int32  rows = 5;
}
message SpawnTerminalSessionResponse {
  TerminalSession session = 1;
}
message TerminalSession {
  string pty_id = 1;
  string connection_id = 2;
  string cwd = 3;
  int64  created_at_unix_ms = 4;
  int64  last_active_at_unix_ms = 5;
}

message ResizeTerminalSessionRequest { string pty_id = 1; int32 cols = 2; int32 rows = 3; }
message KillTerminalSessionRequest   { string pty_id = 1; }
message StopTerminalProcessRequest  { string pty_id = 1; } // sends an interrupt signal to the pty's foreground process
message ListTerminalSessionsRequest { string connection_id = 1; } // empty = all sessions for the caller's tenant
message ListTerminalSessionsResponse { repeated TerminalSession sessions = 1; }

message WaitTerminalSessionRequest  {
  string pty_id = 1;
  int32  timeout_ms = 2; // capped server-side, see Non-functional notes below
}
message WaitTerminalSessionResponse { bool exited = 1; int32 exit_code = 2; bool timed_out = 3; }

message FocusTerminalSessionRequest { string pty_id = 1; }

message GetTerminalAgentStatusRequest  { string pty_id = 1; }
message GetTerminalAgentStatusResponse {
  bool   agent_running = 1;     // answers both terminal.agentStatus and terminal.isRunningAgent
  string agent_kind = 2;        // best-effort, e.g. "claude" | "codex" | "" if unknown — see below
  bool   ready_for_input = 3;   // agentStatus's richer question: is it idle-and-ready, not just alive
}

message InspectTerminalProcessRequest  { string pty_id = 1; }
message InspectTerminalProcessResponse {
  bool   known = 1;    // false when the agent can't answer (see below) — an honest "unknown", not a fabricated zero value
  int32  pid = 2;
  string command = 3;
  string cwd = 4;
}

message PtyClientFrame {
  oneof frame {
    AttachToSession attach = 1;  // first frame only: which pty_id this stream carries I/O for
    PtyInput        input  = 2;  // terminal.send
    PtyResize       resize = 3;  // low-latency in-stream resize (alternative to the unary RPC above)
  }
}
message AttachToSession { string pty_id = 1; }
message PtyInput  { bytes data = 1; }
message PtyResize { int32 cols = 1; int32 rows = 2; }

message PtyServerFrame {
  oneof frame {
    PtyOutput out    = 1;
    PtyExited exited = 2;
  }
}
message PtyOutput { bytes data = 1; }
message PtyExited { int32 exit_code = 1; }
```

All additive — `buf breaking` passes per `08-inter-service-communication.md`.
`AttachPty` is the one non-unary RPC in this service's surface so far;
consistent with `05-data-architecture.md`/`08-inter-service-communication.md`'s
general allowance for server- and client-streaming where the interaction
shape genuinely needs it (this is the same shape `workflow-service.md`'s
`StreamExecutionEvents` and `notification-service`'s
`StreamNotifications` already use elsewhere in this system).

### `SpawnTerminalSessionRequest.connection_id` can legitimately be empty — but rarely, in this deployment

BUG-029's dispatch-model section is explicit that backend-go's server
deployment has **no local-shell path** (`server-pty-controller.ts:66-71`'s
"no local shell" behavior is inherited as a deployment fact, not a design
choice for backend-go to relitigate). `connection_id` stays optional in
the message purely for symmetry with `infra-fleet-service.md`'s own
`ProviderRegistryEntry` model (which does define a `local` provider kind)
and for local/dev deployments — but `SpawnTerminalSession`'s usecase
should reject an empty `connection_id` with `TERMINAL_NO_LOCAL_SHELL` in
the server deployment profile (a config flag, not a hardcoded rejection,
so local/dev mode isn't permanently cut off).

---

## Design — domain model

```go
// internal/domain/terminal_session.go — mirrors infra-fleet-service.md §4's
// TerminalSession entity: "ptyId, owning connectionId, worktree/cwd
// context, created-at, last-active-at. Holds no PTY bytes."
type TerminalSession struct {
    PtyID        string
    ConnectionID string // empty = host-local
    Cwd          string
    CreatedAt    time.Time
    LastActiveAt time.Time
    ClosedAt     *time.Time
}

func (s *TerminalSession) Touch(now time.Time) { s.LastActiveAt = now } // backs FocusTerminalSession
```

No new invariant-bearing type is needed for agent-status/inspect —
those are point-in-time relay answers, not persisted state, consistent
with `infra-fleet-service.md` §4's "Holds no PTY bytes" framing extended
to "holds no process-introspection state either."

`terminal_sessions` (already specified, `infra-fleet-service.md:271-281`)
needs **no schema change** — `pty_id`/`connection_id`/`cwd`/`created_at`/
`last_active_at`/`closed_at` cover every field this design needs.
`FocusTerminalSession` is implemented as a bump of `last_active_at`
(`Touch`, above) rather than a new "focused" column — it exists to keep a
pane a user is actively looking at from being evicted by whatever
idle-session reaper enforces §8's `MAX_CONCURRENT_STREAMS`-style backpressure
cap, not to track UI focus state durably.

---

## Design — `usecase/` layer

One usecase per RPC, per `03-clean-architecture-guidelines.md`'s "mirrors
the granularity of today's RPC methods" convention. The port every one of
them depends on:

```go
// internal/usecase/ports.go
type TerminalSessionRepository interface {
    Create(ctx context.Context, s domain.TerminalSession) error
    Get(ctx context.Context, tenantID, ptyID string) (domain.TerminalSession, bool, error)
    ListByConnection(ctx context.Context, tenantID, connectionID string) ([]domain.TerminalSession, error)
    Touch(ctx context.Context, ptyID string, at time.Time) error
    MarkClosed(ctx context.Context, ptyID string, at time.Time) error
}

// PtyEvent is what DevServerAgentClient.StreamPty yields — output bytes or
// a terminal exit, tagged by ptyId so one Client can multiplex many
// sessions over one underlying agent connection (per session.go's existing
// one-session-per-dev-server model, see below).
type PtyEvent struct {
    PtyID    string
    Output   []byte // nil when Exited is set
    Exited   bool
    ExitCode int32
}

// DevServerAgentClient grows PTY-specific methods alongside the existing
// Exec/Health (client.go:241,267) — see "adapter/devserveragent" section
// below for why these are new methods, not calls through generic Exec.
type DevServerAgentClient interface {
    Exec(ctx context.Context, devServer domain.DevServer, method string, params map[string]any) (map[string]any, error) // existing
    Health(ctx context.Context, devServer domain.DevServer) (bool, error)                                              // existing
    SpawnPty(ctx context.Context, devServer domain.DevServer, cwd, shell string, cols, rows int32) (ptyID string, err error)
    WritePty(ctx context.Context, devServer domain.DevServer, ptyID string, data []byte) error
    ResizePty(ctx context.Context, devServer domain.DevServer, ptyID string, cols, rows int32) error
    KillPty(ctx context.Context, devServer domain.DevServer, ptyID string) error
    // StreamPty subscribes to output/exit notifications for one ptyId.
    // Returned channel closes when ctx is cancelled or the underlying
    // agent session drops — the usecase layer, not this port, decides
    // whether that's a client-visible error.
    StreamPty(ctx context.Context, devServer domain.DevServer, ptyID string) (<-chan PtyEvent, error)
    AgentStatus(ctx context.Context, devServer domain.DevServer, ptyID string) (running bool, kind string, ready bool, err error)
    InspectProcess(ctx context.Context, devServer domain.DevServer, ptyID string) (known bool, pid int32, command, cwd string, err error)
}
```

```go
// internal/usecase/spawn_terminal_session.go
func (uc *SpawnTerminalSession) Execute(ctx context.Context, in SpawnTerminalSessionInput) (domain.TerminalSession, error) {
    if in.ConnectionID == "" && uc.serverDeployment {
        // BUG-029's dispatch-model finding, enforced here — see doc above.
        return domain.TerminalSession{}, apperrors.New(apperrors.KindFailedPrecondition,
            "TERMINAL_NO_LOCAL_SHELL", "this server only supports Dev Server- or SSH-backed terminals", nil)
    }
    devServer, err := uc.connections.ResolveDevServer(ctx, in.ConnectionID) // reuses ResolveConnection's existing lookup
    if err != nil {
        return domain.TerminalSession{}, apperrors.New(apperrors.KindNotFound, "TERMINAL_CONNECTION_NOT_FOUND", "connection not found", err)
    }
    ptyID, err := uc.agent.SpawnPty(ctx, devServer, in.Cwd, in.Shell, in.Cols, in.Rows)
    if err != nil {
        return domain.TerminalSession{}, apperrors.New(apperrors.KindInternal, "TERMINAL_SPAWN_FAILED", "failed to spawn terminal session", err)
    }
    session := domain.TerminalSession{PtyID: ptyID, ConnectionID: in.ConnectionID, Cwd: in.Cwd, CreatedAt: uc.clock.Now(), LastActiveAt: uc.clock.Now()}
    if err := uc.sessions.Create(ctx, session); err != nil {
        // Session exists agent-side but bookkeeping failed — kill it rather
        // than leaking an orphaned PTY the fleet has no record of.
        _ = uc.agent.KillPty(ctx, devServer, ptyID)
        return domain.TerminalSession{}, apperrors.New(apperrors.KindInternal, "TERMINAL_BOOKKEEPING_FAILED", "failed to record terminal session", err)
    }
    return session, nil
}
```

```go
// internal/usecase/attach_pty.go — the usecase behind AttachPty's gRPC
// handler. Bridges DevServerAgentClient.StreamPty (agent -> here) with a
// caller-provided channel of client frames (here -> agent), enforcing the
// concurrent-session cap from infra-fleet-service.md §8
// ("MAX_CONCURRENT_STREAMS = 16... carry the same ceiling forward").
type AttachPty struct {
    sessions TerminalSessionRepository
    agent    DevServerAgentClient
    limiter  *ConnectionStreamLimiter // per-connectionId semaphore, cap 16
}

func (uc *AttachPty) Execute(ctx context.Context, ptyID string, clientFrames <-chan PtyClientFrame, serverFrames chan<- PtyServerFrame) error {
    session, found, err := uc.sessions.Get(ctx, tenant.RequireTenantIDOrEmpty(ctx), ptyID)
    if err != nil || !found {
        return apperrors.New(apperrors.KindNotFound, "TERMINAL_SESSION_NOT_FOUND", "terminal session not found", err)
    }
    release, err := uc.limiter.Acquire(session.ConnectionID)
    if err != nil {
        return apperrors.New(apperrors.KindResourceExhausted, "TERMINAL_TOO_MANY_STREAMS", "too many concurrent terminal streams for this connection", err)
    }
    defer release()

    devServer, _ := uc.connections.ResolveDevServer(ctx, session.ConnectionID)
    events, err := uc.agent.StreamPty(ctx, devServer, ptyID)
    if err != nil {
        return apperrors.New(apperrors.KindInternal, "TERMINAL_STREAM_FAILED", "failed to open pty stream", err)
    }

    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case frame, ok := <-clientFrames:
            if !ok {
                return nil
            }
            switch f := frame.(type) {
            case PtyInputFrame:
                if err := uc.agent.WritePty(ctx, devServer, ptyID, f.Data); err != nil {
                    return apperrors.New(apperrors.KindInternal, "TERMINAL_WRITE_FAILED", "failed to write to terminal", err)
                }
                _ = uc.sessions.Touch(ctx, ptyID, uc.clock.Now())
            case PtyResizeFrame:
                _ = uc.agent.ResizePty(ctx, devServer, ptyID, f.Cols, f.Rows)
            }
        case ev, ok := <-events:
            if !ok {
                return nil
            }
            if ev.Exited {
                serverFrames <- PtyServerFrame{Exited: &PtyExited{ExitCode: ev.ExitCode}}
                return nil
            }
            serverFrames <- PtyServerFrame{Out: &PtyOutput{Data: ev.Output}}
        }
    }
}
```

```go
// internal/usecase/stop_terminal_process.go — "stop" is an interrupt, not
// a teardown (distinct from KillTerminalSession/terminal.close). No new
// agent-side capability needed: it writes the interrupt control byte
// (0x03, Ctrl+C) through the SAME WritePty path terminal.send uses — a
// convenience RPC over an existing primitive, not a new agent method.
// This keeps the design inside Option A's "agent doesn't change" bound.
func (uc *StopTerminalProcess) Execute(ctx context.Context, ptyID string) error {
    // ... resolve session/devServer as above ...
    return uc.agent.WritePty(ctx, devServer, ptyID, []byte{0x03})
}
```

```go
// internal/usecase/wait_terminal_session.go — bounded blocking poll.
// Deadlines are mandatory (08-inter-service-communication.md); the
// caller's timeout_ms is honored but capped (default max 30s, documented
// override, same posture as workflow-service's 30-minute step timeout
// being an explicit exception to the 5s intra-cluster default).
func (uc *WaitTerminalSession) Execute(ctx context.Context, ptyID string, timeout time.Duration) (exited bool, exitCode int32, timedOut bool, err error) {
    timeout = min(timeout, maxWaitTimeout) // 30s default cap
    waitCtx, cancel := context.WithTimeout(ctx, timeout)
    defer cancel()
    devServer, _ := uc.connections.ResolveDevServer(ctx, /* session's connectionId */ "")
    events, err := uc.agent.StreamPty(waitCtx, devServer, ptyID)
    if err != nil {
        return false, 0, false, err
    }
    for {
        select {
        case <-waitCtx.Done():
            return false, 0, true, nil
        case ev, ok := <-events:
            if !ok {
                return false, 0, true, nil
            }
            if ev.Exited {
                return true, ev.ExitCode, false, nil
            }
        }
    }
}
```

`GetTerminalAgentStatus`/`InspectTerminalProcess` are thin
`uc.agent.AgentStatus`/`uc.agent.InspectProcess` wrappers with a tenant/
session lookup guard, same shape as `StopTerminalProcess` above.

### `terminal.isRunningAgent` reuses `GetTerminalAgentStatus`, not a second RPC

BUG-029 lists `terminal.agentStatus` and `terminal.isRunningAgent` as two
distinct frontend methods with two distinct call sites
(`active-agent-terminal-send-readiness.ts` for both;
`active-agent-note-target.ts` for the latter only), but they're the same
underlying question — "is an agent CLI alive/ready in this pty" — asked at
different granularity. One RPC, two `wscompat` channels, is the same
"reusable port, not an indirection" principle `workflow-service.md` §3.1
uses for `ExecuteAdHocStep`: `terminal.isRunningAgent` projects
`agent_running` alone; `terminal.agentStatus` returns the fuller
`{agent_running, agent_kind, ready_for_input}` shape.

### `InspectTerminalProcess`/`GetTerminalAgentStatus` degrade honestly when the agent can't answer

BUG-029 cites `agent/src/relay/pty-agent-bridge.ts` as existing
infrastructure — the agent side plausibly already tracks "is an agent
process the pty's foreground job" for its own purposes (its filename
implies exactly this), which is why this design assumes `AgentStatus`
relays to it rather than inventing new agent state. `InspectProcess` is a
weaker bet — no cited agent file suggests a general process-introspection
RPC exists — so `InspectTerminalProcessResponse.known` is a first-class
`false` case: if the underlying `Exec`/relay call the adapter makes
returns "unknown method," the usecase returns `known:false` rather than a
gRPC error, following `channels.go`'s existing "best-effort... honest
placeholder, not fabricated data" convention (`channels.go:6-14`,
`devServerHost`'s field-mapping doc comment). Confirm against
`agent/src/relay/pty-agent-bridge.ts`'s actual RPC surface before
implementation — flagged, not assumed.

---

## Design — `adapter/devserveragent` — the gRPC-over-existing-wire-protocol adapter

Per `08-inter-service-communication.md`'s "Talking to the Dev Server
Agent" section, **Option A is the explicit default recommendation**:
preserve the existing wire protocol, no `agent/` changes. This design
follows it exactly — every new `DevServerAgentClient` method above is
implemented as a call through the **same** 13-byte-framed JSON-RPC
transport `client.go`'s existing `Exec`/`Health` already use
(`client.go:241-258,267-274`), not a new protocol:

```go
// internal/adapter/devserveragent/methods.go — typed wrappers, per
// infra-fleet-service.md §6's package-layout note ("methods.go — typed
// wrappers for the specific agent RPC methods this service calls:
// pty.spawn/write/resize/kill..."). This is where the Stack A vs Stack B
// method-name divergence (§10: "pty.create vs pty.spawn") is absorbed —
// one place, not per call site, per that section's explicit warning.
func (c *Client) SpawnPty(ctx context.Context, devServer domain.DevServer, cwd, shell string, cols, rows int32) (string, error) {
    method := c.ptyMethodName(devServer, "spawn") // resolves to "pty.spawn" or "pty.create" per the session's negotiated Stack
    result, err := c.Exec(ctx, devServer, method, map[string]any{"cwd": cwd, "shell": shell, "cols": cols, "rows": rows})
    if err != nil {
        return "", err
    }
    ptyID, _ := result["ptyId"].(string)
    return ptyID, nil
}

func (c *Client) WritePty(ctx context.Context, devServer domain.DevServer, ptyID string, data []byte) error {
    _, err := c.Exec(ctx, devServer, c.ptyMethodName(devServer, "write"),
        map[string]any{"ptyId": ptyID, "data": base64.StdEncoding.EncodeToString(data)})
    return err
}
```

`StreamPty` is the one genuinely new capability `client.go` needs — today
it has no channel-based subscription primitive, only `Exec`'s
request/response `sess.call` (`client.go:242-247`). The agent already
*emits* PTY output as JSON-RPC **notifications** (no ID, no response
expected) over the same framed connection per-`ptyId` — this is exactly
what `agent/src/relay/pty-handler.ts`'s output-forwarding side does today,
independent of this solution. `Stream` adds a notification-demux layer
`session.go`'s read loop feeds into:

```go
// internal/adapter/devserveragent/client.go — new method
func (c *Client) StreamPty(ctx context.Context, devServer domain.DevServer, ptyID string) (<-chan usecase.PtyEvent, error) {
    sess, err := c.getOrCreateSession(ctx, devServer)
    if err != nil {
        return nil, err
    }
    // subscribeNotifications(ptyID) registers a filtered channel against
    // session.go's existing readLoop — every inbound JSON-RPC notification
    // whose params.ptyId matches is routed here instead of being dropped
    // (today's readLoop only resolves pending request/response pairs by
    // ID; notifications with no matching pending call are currently
    // discarded — this is the one real code change to session.go itself,
    // additive, doesn't touch request/response handling).
    return sess.subscribeNotifications(ctx, ptyID), nil
}
```

This is the one place this design touches `session.go`'s read loop, and
it's additive (a new notification-routing branch, not a change to the
existing request/response path) — consistent with "only this service
needs a new client implementation of a protocol that already exists"
(`infra-fleet-service.md:551-552`).

### Stack A/B divergence is absorbed once, per `infra-fleet-service.md` §10's explicit warning

`ptyMethodName(devServer, verb)` is a single lookup table keyed by which
Part (A: local WS-connected dispatcher, B: SSH-deployed `RelayDispatcher`)
the session negotiated at handshake — `handshake.go` already knows this
per `infra-fleet-service.md:332`. Every `pty.*` call site in `methods.go`
goes through it; no call site hardcodes `"pty.spawn"` or `"pty.create"`
directly, closing exactly the class of bug §10 flags
(`gaps-and-findings.md`'s TS-side divergence bugs) by construction.

---

## Design — `wscompat` wiring (`api-gateway`) — reusing SOL-035's push-bridge pattern

`terminal.create` is shaped like SOL-035's `notifications.subscribe`
precedent: an `invoke` call whose job is to both return a value (the new
`ptyId`) **and** start a stream. Register it as a
`StreamHandler`-and-ack, exactly SOL-035's pattern
(`push_bridge.go`'s dispatch-switch addition):

```go
func registerTerminalChannels(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
    // terminal.create: ordinary invoke response (ptyId) AND registers a
    // stream — the ack race is handled the same way SOL-035's design
    // handles notifications.subscribe: ack first via the normal invoke
    // path, THEN start pipePush in a goroutine (push_bridge.go:97-114).
    r.RegisterStream("terminal.create", func(ctx context.Context, id Identity, args []json.RawMessage) (any, <-chan PushEvent, error) {
        in, err := decodeArg[createArgs](args, 0)
        if err != nil { return nil, nil, err }
        ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
        resp, err := client.SpawnTerminalSession(ctx, &infrafleetv1.SpawnTerminalSessionRequest{
            ConnectionId: in.ConnectionID, Cwd: in.Cwd, Shell: in.Shell, Cols: in.Cols, Rows: in.Rows,
        })
        if err != nil { return nil, nil, err }
        ptyID := resp.GetSession().GetPtyId()

        // Open AttachPty now (not lazily on first send) — output must start
        // flowing immediately even before the caller's first terminal.send.
        stream, err := client.AttachPty(ctx)
        if err != nil { return nil, nil, err }
        if err := stream.Send(&infrafleetv1.PtyClientFrame{Frame: &infrafleetv1.PtyClientFrame_Attach{Attach: &infrafleetv1.AttachToSession{PtyId: ptyID}}}); err != nil {
            return nil, nil, err
        }
        r.terminalStreams.Register(ptyID, stream) // per-connection registry, see below — lets terminal.send/resize find this stream

        events := make(chan PushEvent)
        go pipeAttachPtyToPush(ctx, stream, ptyID, events) // translates PtyServerFrame -> PushEvent{Channel: "terminal.output", Args: {ptyId, data}}
        return resp.GetSession(), events, nil
    })

    r.Register("terminal.send", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
        in, err := decodeArg[sendArgs](args, 0) // {ptyId, data (base64)}
        if err != nil { return nil, err }
        stream, ok := r.terminalStreams.Get(in.PtyID)
        if !ok {
            return nil, fmt.Errorf("terminal session %q has no open stream on this connection", in.PtyID)
        }
        return nil, stream.Send(&infrafleetv1.PtyClientFrame{Frame: &infrafleetv1.PtyClientFrame_Input{Input: &infrafleetv1.PtyInput{Data: in.Data}}})
    })

    r.Register("terminal.close", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
        in, err := decodeArg[closeArgs](args, 0)
        if err != nil { return nil, err }
        ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
        _, err = client.KillTerminalSession(ctx, &infrafleetv1.KillTerminalSessionRequest{PtyId: in.PtyID})
        r.terminalStreams.Close(in.PtyID) // tears down the AttachPty stream this connection opened for it
        return map[string]bool{"ok": err == nil}, err
    })

    // terminal.stop, terminal.list, terminal.wait, terminal.focus,
    // terminal.agentStatus, terminal.isRunningAgent, terminal.inspectProcess
    // are ordinary unary wrappers, same shape as registerGitChannels
    // (channels.go:221-252) — omitted here for brevity, each a direct
    // 1:1 mapping onto the RPCs defined above.
}
```

`r.terminalStreams` is a **per-WS-connection** registry (`map[ptyId]
infrafleetv1.InfraFleetService_AttachPtyClient`, mutex-guarded), created
once in `ServeHTTP` alongside the `writeMu` SOL-035 already introduces,
and torn down when the connection closes — exactly the "no new
connection registry... scoped to `ServeHTTP`'s existing per-connection
goroutine group, not a cross-connection registry" principle SOL-035
establishes for its own push-piping, applied here to the write-half too
(SOL-035 only needed a registry-free design because
`StreamNotifications` has no client→server direction; `AttachPty` does,
so a small per-connection lookup table is the minimal addition beyond
SOL-035's shape, not a rewrite of it).

`pipeAttachPtyToPush` is a thin translator feeding the exact same
`pipePush` SOL-035 defines (`push_bridge.go:78-94`) — `PtyServerFrame{Out}`
becomes `PushEvent{Channel: "terminal.output", Args: {ptyId, data:
base64}}`; `PtyServerFrame{Exited}` becomes `PushEvent{Channel:
"terminal.exited", Args: {ptyId, exitCode}}` and closes the channel. No
new push-delivery mechanism — SOL-035's `pipePush`/`writeMu`-serialization
is reused verbatim.

`RegisterStream` is SOL-035's proposed `StreamHandlers` map
(`push_bridge.go:66,104`) extended to also carry an ack value (SOL-035's
sketch returns only `(<-chan PushEvent, error)`; `terminal.create` needs
to additionally return the spawned `TerminalSession` as the invoke's own
`ResultMessage.Result`) — a small, backward-compatible signature widening
of that map's value type, not a second registry.

---

## Non-functional notes carried from `infra-fleet-service.md`

- **§8's `MAX_CONCURRENT_STREAMS = 16` cap** is enforced in `AttachPty`'s
  usecase (`ConnectionStreamLimiter`, above), not just documented — a
  runaway frontend opening many panes against one `connectionId` gets a
  typed `TERMINAL_TOO_MANY_STREAMS` rather than silently degrading the
  underlying agent connection for every other session sharing it.
- **Deadlines**: `WaitTerminalSession`'s cap (default 30s, documented
  override) and `AttachPty`'s open-ended lifetime (bound to the WS
  connection's own lifetime, not the 5s intra-cluster default) both need
  explicit call-site documentation per `08-inter-service-communication.md`'s
  "overridable per call site with a documented reason" rule.
- **`connectionId` resolution caching**: `SpawnTerminalSession`/`AttachPty`
  resolve `connectionId → DevServer` via the same path `ResolveConnection`
  already uses (`infra-fleet-service.md` §7's sequence diagram) — no new
  resolution logic, reuses the existing in-process `ProviderRegistryEntry`
  cache.

---

## Test plan

- `services/infra-fleet-service/internal/usecase/spawn_terminal_session_test.go`
  — fake `DevServerAgentClient`/`TerminalSessionRepository`: happy path;
  bookkeeping-write failure kills the already-spawned agent-side PTY
  (regression guard against orphaned sessions); `connectionId=""` in
  server-deployment mode returns `TERMINAL_NO_LOCAL_SHELL`.
- `services/infra-fleet-service/internal/usecase/attach_pty_test.go` —
  fake `StreamPty`/`WritePty`: input frames write through, resize frames
  resize, an `Exited` event ends the loop and emits exactly one
  `PtyExited` server frame; the 17th concurrent stream on one
  `connectionId` is rejected with `TERMINAL_TOO_MANY_STREAMS`.
- `services/infra-fleet-service/internal/usecase/wait_terminal_session_test.go`
  — exits before timeout returns `exited:true`; no exit within the capped
  window returns `timed_out:true`, never blocks past the cap.
- `services/infra-fleet-service/internal/adapter/devserveragent/methods_test.go`
  — `ptyMethodName` resolves `"spawn"` to `"pty.spawn"` for a Stack-A
  session and `"pty.create"` for a Stack-B session (or whatever the real
  negotiated names turn out to be — verify against `agent/src/relay/`
  before finalizing); `StreamPty` demuxes notifications by `ptyId`,
  ignoring ones for other sessions on the same shared agent connection.
- `services/api-gateway/internal/adapter/wscompat/channels_terminal_test.go`
  — `terminal.create` against a fake `InfraFleetServiceClient` acks with a
  `ptyId` AND subsequently delivers `terminal.output` push frames from a
  fake `AttachPty` stream, interleaved correctly with a concurrent
  `terminal.send` write (mirrors SOL-035's own `writeMu`-sharing
  regression test); `terminal.send` against an unknown `ptyId` (no
  registry entry — e.g., a stale pane after reconnect) returns a clear
  error, not a panic; `terminal.close` tears down the registry entry.
- Integration test against a fake Dev Server Agent WS server (mirrors
  `provisioner_test.go`'s `fakeSSHServer` pattern in
  `sshrelay/provisioner_test.go`, adapted for the direct-websocket/
  relay-websocket transports) — end-to-end `terminal.create` →
  `terminal.send` → observe echoed output via `terminal.output` push →
  `terminal.close`.

## References

- `specs/backend-go/tdd/services/infra-fleet-service.md:123-137` (§3) — existing terminal RPC sketch this solution extends
- `specs/backend-go/tdd/services/infra-fleet-service.md:360-366` (§7) — the "server-streaming terminal-data endpoint" this solution names and adds as `AttachPty`
- `specs/backend-go/tdd/services/infra-fleet-service.md:293-348` (§6) — `adapter/devserveragent/` package layout, `methods.go`'s typed-wrapper convention
- `specs/backend-go/tdd/services/infra-fleet-service.md:446-484` (§8) — `MAX_CONCURRENT_STREAMS` backpressure cap, deadline rules
- `specs/backend-go/tdd/services/infra-fleet-service.md:560-573` (§10) — Option A decision, Stack A/B method-name divergence warning
- `specs/backend-go/tdd/architecture/08-inter-service-communication.md` — "Talking to the Dev Server Agent," Option A vs B
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto:10-31` — current `InfraFleetService` surface (no PTY RPCs)
- `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/client.go:76-102,241-274` — `Client.Exec`/`Health`, the existing request/response pattern `SpawnPty`/`WritePty`/etc. extend
- `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/session.go:152` — only existing PTY-adjacent reference in this service today
- `backend-go/services/api-gateway/internal/adapter/wsbridge/handler.go` — existing gRPC-stream→WS bridge precedent SOL-035 generalizes and this solution reuses
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:64-82,221-252` — `RegisterRealChannels`, `registerGitChannels` pattern for the unary handlers
- [BUG-029](../BUG-029-terminal-channels-not-implemented.md) — full findings this solution builds on, including agent-side PTY infrastructure citations (`pty-handler.ts`, `pty-agent-bridge.ts`, etc.)
- [BUG-035](../BUG-035-ws-server-push-not-implemented.md) / [SOL-035](./SOL-035-ws-server-push-bridge.md) — the push-bridge pattern this solution's `terminal.output`/`terminal.exited` delivery reuses; SOL-035 itself named this solution as its blocking dependency for terminal channels
