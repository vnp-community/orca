# TASK-MB-04-01: Add `last_output_preview` to `TerminalSession`/`GetTerminalAgentStatusResponse`

**From Solution:** SOL-MB-04
**Priority:** P0
**Service:** `infra-fleet-service`
**File:** `backend-go/proto/orca/infrafleet/v1/infrafleet.proto`
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

BR-MB-15's 500-char truncated output preview needs a wire field. Confirmed
current field numbers: `TerminalSession` has fields 1-5 in use (next
available is 6, not the SOL's originally-sketched 10);
`GetTerminalAgentStatusResponse` has fields 1-3 in use (next available is
4, matching the SOL). Additive-only.

## Changes to make

In `TerminalSession` (around line 320-326):

```protobuf
message TerminalSession {
  string pty_id = 1;
  string connection_id = 2;
  string cwd = 3;
  int64  created_at_unix_ms = 4;
  int64  last_active_at_unix_ms = 5;
  string last_output_preview = 6; // BR-MB-15: truncated to 500 chars server-side, never a raw dump — see terminal_session.go's TruncatedForMobile
}
```

In `GetTerminalAgentStatusResponse` (around line 343-347):

```protobuf
message GetTerminalAgentStatusResponse {
  bool   agent_running = 1;
  string agent_kind = 2;
  bool   ready_for_input = 3;
  string last_output_preview = 4;
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
