# TASK-180: Add `AttachPty` streaming RPC + full terminal proto surface + `TerminalSession` domain model

**From Solution:** SOL-029 (design part 1: "the `AttachPty` streaming RPC design + proto")
**Priority:** P0 — everything else in this solution depends on generated stubs and the domain type from this
**Service:** `infra-fleet-service`
**File:** `backend-go/proto/orca/infrafleet/v1/infrafleet.proto`, `services/infra-fleet-service/internal/domain/terminal_session.go` (new)
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

BUG-029 confirmed **zero** PTY-shaped RPC exists anywhere in backend-go's
proto surface. `infra-fleet-service.md` §3 sketches control-plane RPCs
(`SpawnTerminalSession`/`RouteTerminalWrite`/`ResizeTerminalSession`/
`KillTerminalSession`/`ListTerminalSessions`) but §7 describes a 6th piece
— a dedicated PTY-data **streaming** RPC — that §3's list never actually
names, and names `RouteTerminalWrite` for routing metadata only (not
per-byte input). This task adds the real surface: a single **bidirectional**
streaming RPC, `AttachPty`, carrying both input+resize (client→server) and
output+exit (server→client) — replacing the never-implemented
`RouteTerminalWrite` sketch — plus the 9 unary lifecycle RPCs BUG-029's 10
`terminal.*` frontend methods need. All additive; `buf breaking` passes.

`AttachPty` is the one non-unary RPC this service will have; consistent
with `08-inter-service-communication.md`'s general allowance for streaming
where the interaction genuinely needs it (same shape `workflow-service`'s
`StreamExecutionEvents` and `notification-service`'s `StreamNotifications`
already use elsewhere).

## Changes to make

### `backend-go/proto/orca/infrafleet/v1/infrafleet.proto`

Add to the `InfraFleetService` block (after the existing `Relay` RPC, before
the closing `}`):

```protobuf
  // --- Terminal/PTY lifecycle (control-plane, unary) ---
  rpc SpawnTerminalSession(SpawnTerminalSessionRequest) returns (SpawnTerminalSessionResponse);
  rpc ResizeTerminalSession(ResizeTerminalSessionRequest) returns (google.protobuf.Empty);
  rpc KillTerminalSession(KillTerminalSessionRequest) returns (google.protobuf.Empty);      // terminal.close
  rpc StopTerminalProcess(StopTerminalProcessRequest) returns (google.protobuf.Empty);      // terminal.stop — interrupt, not teardown
  rpc ListTerminalSessions(ListTerminalSessionsRequest) returns (ListTerminalSessionsResponse);
  rpc WaitTerminalSession(WaitTerminalSessionRequest) returns (WaitTerminalSessionResponse); // terminal.wait — bounded blocking poll
  rpc FocusTerminalSession(FocusTerminalSessionRequest) returns (google.protobuf.Empty);     // terminal.focus — bookkeeping touch
  rpc GetTerminalAgentStatus(GetTerminalAgentStatusRequest) returns (GetTerminalAgentStatusResponse); // backs BOTH terminal.agentStatus and terminal.isRunningAgent
  rpc InspectTerminalProcess(InspectTerminalProcessRequest) returns (InspectTerminalProcessResponse); // best-effort — see message doc comment

  // --- Terminal/PTY I/O ---
  // The "server-streaming terminal-data endpoint" infra-fleet-service.md §7
  // names but §3 never enumerates. Bidirectional, not server-streaming-only:
  // §3's RouteTerminalWrite only routes control-plane metadata, never per-
  // byte input, so a client->server data channel was still missing — this
  // RPC is opened once per terminal.create by api-gateway's wscompat bridge
  // (TASK-186) and piped into `push` frames via TASK-012's pipePush.
  rpc AttachPty(stream PtyClientFrame) returns (stream PtyServerFrame);
```

Add these messages at the bottom of the file:

```protobuf
message SpawnTerminalSessionRequest {
  string connection_id = 1;  // empty = host-local; rejected in server-deployment mode, see usecase.SpawnTerminalSession
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
message StopTerminalProcessRequest   { string pty_id = 1; } // sends an interrupt signal to the pty's foreground process
message ListTerminalSessionsRequest  { string connection_id = 1; } // empty = all sessions for the caller's tenant
message ListTerminalSessionsResponse { repeated TerminalSession sessions = 1; }

message WaitTerminalSessionRequest  {
  string pty_id = 1;
  int32  timeout_ms = 2; // capped server-side (default max 30s, see usecase.WaitTerminalSession)
}
message WaitTerminalSessionResponse { bool exited = 1; int32 exit_code = 2; bool timed_out = 3; }

message FocusTerminalSessionRequest { string pty_id = 1; }

message GetTerminalAgentStatusRequest  { string pty_id = 1; }
message GetTerminalAgentStatusResponse {
  bool   agent_running = 1;     // answers both terminal.agentStatus and terminal.isRunningAgent
  string agent_kind = 2;        // best-effort, e.g. "claude" | "codex" | "" if unknown
  bool   ready_for_input = 3;   // agentStatus's richer question: is it idle-and-ready, not just alive
}

message InspectTerminalProcessRequest  { string pty_id = 1; }
message InspectTerminalProcessResponse {
  bool   known = 1;    // false when the agent can't answer — an honest "unknown", not a fabricated zero value
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

`google.protobuf.Empty` is already imported by this proto file (used by
`ResolveConnection`'s neighbors) — if it is not, add
`import "google/protobuf/empty.proto";` alongside the file's other imports.

### `services/infra-fleet-service/internal/domain/terminal_session.go` (new)

```go
// Package domain (terminal_session.go) — mirrors infra-fleet-service.md
// §4's TerminalSession entity: "ptyId, owning connectionId, worktree/cwd
// context, created-at, last-active-at. Holds no PTY bytes." No new
// invariant-bearing type is needed for agent-status/inspect — those are
// point-in-time relay answers, not persisted state, extending §4's "Holds
// no PTY bytes" framing to "holds no process-introspection state either."
package domain

import "time"

// TerminalSession is metadata about one PTY session — explicitly not the
// PTY's byte stream itself (that never touches Postgres). ConnectionID
// empty means host-local (rare; see SpawnTerminalSessionRequest's doc
// comment).
type TerminalSession struct {
	PtyID        string
	ConnectionID string
	Cwd          string
	CreatedAt    time.Time
	LastActiveAt time.Time
	ClosedAt     *time.Time
}

// Touch bumps LastActiveAt — backs FocusTerminalSession. Exists to keep a
// pane a user is actively looking at from being evicted by whatever
// idle-session reaper enforces the MAX_CONCURRENT_STREAMS-style
// backpressure cap (infra-fleet-service.md §8), not to track UI focus
// state durably.
func (s *TerminalSession) Touch(now time.Time) { s.LastActiveAt = now }
```

## Schema note (no migration needed)

`terminal_sessions` is already specified (`infra-fleet-service.md:271-281`)
with exactly the columns this domain type needs
(`pty_id`/`connection_id`/`cwd`/`created_at`/`last_active_at`/`closed_at`).
Confirm the migration already exists under
`services/infra-fleet-service/migrations/`; if it doesn't, add one before
TASK-181 (the Postgres repository task) — flagged here, not assumed.

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
go build ./services/infra-fleet-service/internal/domain/...
```

Expected: clean build, `buf breaking` reports only additions (10 new RPCs
— 1 streaming, 9 unary — and their messages).
