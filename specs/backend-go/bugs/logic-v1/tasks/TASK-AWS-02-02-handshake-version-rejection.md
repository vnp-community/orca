# TASK-AWS-02-02: Reject direct-websocket handshakes below `MinAgentVersion` (WS 1008)

**From Solution:** SOL-AWS-02
**Priority:** P1
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/adapter/agentwsserver/server.go`
**Depends on:** TASK-AWS-02-01
**Status:** [x] DONE — `rejectVersion`/`handshakeFailedCode` added, wired into `handleConnection`; 4 new server_test.go cases (below-min rejected, no-version fails open, at-min succeeds, message contains both versions) all green; no 4001-4004 codes present.

---

## Context

Wires `isBelowMinimumVersion` (TASK-AWS-02-01) into `handleConnection`,
after token validation, before `acknowledgeHandshake`. Reuses WS close code
1008 (`websocket.StatusPolicyViolation`) — the same code
`rejectHandshake` already uses for a bad token — disambiguated by message
text, matching the already-settled TS-era decision (no custom 4000-range
close codes). Uses `AgentErrorCode.HandshakeFailed = -33100`
(`agent-wire-protocol.ts:47`), distinct from `authFailedCode = -33101`
already defined at `server.go:31`.

## Changes to make

In `backend-go/services/infra-fleet-service/internal/adapter/agentwsserver/server.go`,
add the new error code constant next to the existing ones (`:29-33`):

```go
const (
	agentHandshakeMethod = "agent.handshake"
	handshakeTimeout     = 20 * time.Second

	authFailedCode    = -33101
	authFailedMessage = "Authentication failed: invalid or unregistered agent token"

	// handshakeFailedCode mirrors AgentErrorCode.HandshakeFailed
	// (agent-wire-protocol.ts:47) — used for the version-mismatch
	// rejection below, distinct from authFailedCode.
	handshakeFailedCode = -33100

	base36Chars = "0123456789abcdefghijklmnopqrstuvwxyz"
)
```

In `handleConnection`, insert the version check right after
`Registry.Consume` succeeds and before `acknowledgeHandshake`
(`server.go:147-157`):

```go
	devServerID, ok := s.Registry.Consume(params.AgentToken)
	if !ok {
		s.rejectHandshake(hctx, conn, req.ID)
		return
	}

	if params.AgentVersion != "" && isBelowMinimumVersion(params.AgentVersion, s.Cfg.MinAgentVersion) {
		s.rejectVersion(hctx, conn, req.ID, params.AgentVersion)
		return
	}

	sessionID := newSessionID()
	// ... unchanged from here ...
```

Add `rejectVersion`, mirroring `rejectHandshake`'s shape:

```go
// rejectVersion sends a JSON-RPC HandshakeFailed error frame, then closes
// the WS with code 1008 — same code rejectHandshake uses for a bad token,
// disambiguated by message text per the settled TS-era decision (see
// SOL-AWS-02): no custom 4000-range close code.
func (s *Server) rejectVersion(ctx context.Context, conn *websocket.Conn, requestID uint32, agentVersion string) {
	msg := fmt.Sprintf("Agent version %s is below the minimum supported version %s. Please update the Orca agent.", agentVersion, s.Cfg.MinAgentVersion)
	resp := devserveragent.JSONRPCResponse{
		JSONRPC: "2.0", ID: requestID,
		Error: &devserveragent.JSONRPCError{Code: handshakeFailedCode, Message: msg},
	}
	if frame, err := devserveragent.EncodeJSONRPCFrame(resp, 1, 0); err == nil {
		_ = conn.Write(ctx, websocket.MessageBinary, frame)
	}
	conn.Close(websocket.StatusPolicyViolation, msg)
}
```

`params.AgentVersion != ""` guards against rejecting older agent builds
that predate the field entirely — fail open on missing data, matching
`firstNonEmpty`'s existing fallback posture (`server.go:198`) and
`isBelowMinimumVersion`'s own empty-string short-circuit
(TASK-AWS-02-01).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/adapter/agentwsserver/...
grep -RIn '400[1-4]' services/infra-fleet-service/internal/adapter/agentwsserver/
```

Expected: clean build/tests; the `grep` for `4001`-`4004` returns nothing —
this is the intended, documented state (no custom close codes), not an
oversight. Add to `server_test.go`: a handshake with `agentVersion` below
`Cfg.MinAgentVersion` is rejected with code 1008 and a message containing
both versions; a handshake with no `agentVersion` field is **not**
rejected; a handshake at or above the minimum succeeds unchanged; the
existing binary-first handshake path stays green (regression guard).
