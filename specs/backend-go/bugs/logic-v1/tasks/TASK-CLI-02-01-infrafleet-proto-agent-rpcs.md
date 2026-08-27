# TASK-CLI-02-01: `infrafleet.proto` — 3 new RPCs for CLI agent access

**From Solution:** SOL-CLI-02
**Priority:** P0 — every other task in this set depends on generated stubs from this
**Service:** `infra-fleet-service` (proto)
**File:** `backend-go/proto/orca/infrafleet/v1/infrafleet.proto`
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

BUG-CLI-02 is a wrapper gap, not a capability gap: `terminal.agentStatus`/`terminal.wait`/`terminal.send`/scrollback are real, gRPC-backed operations already, but reachable only through `wscompat`'s WS bridge. Closing the gap for REST/CLI callers needs three new pieces of surface: worktree->pty resolution (zero new state, composes two existing RPCs server-side), a stateless send (existing sends require a held `AttachPty` stream), and a flat-text scrollback read.

## Changes to make

Add to the `InfraFleetService` service block in `infrafleet.proto` (near the existing terminal RPCs, e.g. after `GetTerminalAgentStatus`):

```protobuf
service InfraFleetService {
  // ... existing RPCs unchanged ...

  // GetAgentTerminalSession resolves worktree_id -> the live TerminalSession
  // whose cwd matches that worktree's path, if one exists. Composes
  // ResolveConnection + ListTerminalSessions server-side so no caller
  // re-derives the match/tie-break logic itself.
  rpc GetAgentTerminalSession(GetAgentTerminalSessionRequest) returns (GetAgentTerminalSessionResponse);

  // SendTerminalInput writes directly to the pty's input, bypassing
  // AttachPty's stream — for stateless (REST/CLI) callers that never
  // attach. GUI callers keep using terminal.send/AttachPty for lower
  // latency; this is not a replacement for that path.
  rpc SendTerminalInput(SendTerminalInputRequest) returns (google.protobuf.Empty);

  // GetTerminalScrollback returns a flat-text capture of a pty's recent
  // output, for callers that want a redirectable string, not multiplex
  // frames.
  rpc GetTerminalScrollback(GetTerminalScrollbackRequest) returns (GetTerminalScrollbackResponse);
}

message GetAgentTerminalSessionRequest  { string worktree_id = 1; }
message GetAgentTerminalSessionResponse {
  bool found = 1;
  TerminalSession session = 2; // unset when found=false
}

message SendTerminalInputRequest { string pty_id = 1; bytes data = 2; }

message GetTerminalScrollbackRequest  { string pty_id = 1; }
message GetTerminalScrollbackResponse {
  string text = 1;
  bool   truncated = 2; // true if the retention bound already dropped
                         // earlier output — an honest signal, not silently
                         // partial data
}
```

`TerminalSession` (used by `GetAgentTerminalSessionResponse`) already exists at `infrafleet.proto:320-326` — no change needed there.

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
grep -n "GetAgentTerminalSession\|SendTerminalInput\|GetTerminalScrollback" proto/gen/go/orca/infrafleet/v1/infrafleet_grpc.pb.go
```

Expected: clean build, `buf breaking` reports no breaking changes, and all three new RPCs appear in the generated `InfraFleetServiceClient`/`InfraFleetServiceServer` interfaces.
