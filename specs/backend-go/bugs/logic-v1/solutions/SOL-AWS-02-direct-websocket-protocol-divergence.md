# SOL-AWS-02: Keep the binary-first handshake (Option A); fix version-mismatch and capacity gaps, don't invent 4001–4004

**Resolves:** [BUG-AWS-02](../BUG-AWS-02-direct-websocket-protocol-diverges-from-spec.md)
**Service:** `infra-fleet-service`
**Affected files (proposed):**
- `backend-go/services/infra-fleet-service/internal/adapter/agentwsserver/server.go` (extended)
- `backend-go/services/infra-fleet-service/internal/adapter/agentwsserver/config.go` or a new `version.go` (new: `MinAgentVersion`, semver-lt check)
- `backend-go/services/infra-fleet-service/internal/adapter/agentwsserver/capacity.go` (new)
- `docs/logic/agent-ws/BL-AWS-02-direct-websocket.md` (**doc fix**, not code — see decision below; out of this solution's write scope, flagged as a required companion PR)
- corresponding `*_test.go` files
**Status:** 📋 Proposed — not yet implemented

---

## The wire-format compatibility decision

**Decision: do not change the transport.** Keep the existing binary-first
`agent.handshake` exchange (agent sends first, 13-byte-framed JSON-RPC, no
`handshake-request` push) exactly as `agentwsserver/server.go` implements
it today. The documented plain-JSON `handshake-request`/`agent.handshake`/
`handshake-ok` exchange and the 4001–4004 close-code table in
`docs/logic/agent-ws/BL-AWS-02-direct-websocket.md` are the side that's
wrong; the doc needs to be fixed to match the running system, not the
other way around.

### Why: the real `agent/` codebase already speaks the binary-first protocol

BUG-AWS-02 frames this as "a third-party agent author who implements the
handshake exactly as documented... cannot connect" — but the actual,
shipping Dev Server Agent this system runs already **doesn't** implement
what the doc describes either, and never has:

- `agent/src/shared/agent-wire-protocol.ts:62-63` — "Params agent sends in
  `agent.handshake` — **first message after WS connected**." No
  `handshake-request` push is ever sent or awaited anywhere in `agent/`.
- `agent/src/relay/agent-connection-direct.ts` (the real direct-websocket
  client) dials out and immediately calls `session.start(ws)`, which
  sends `agent.handshake` unprompted — confirmed by
  `agent/src/relay/agent-connection-stdio.test.ts:123`: "sends
  `agent.handshake` as the very first message once the session starts."
- The frame format is the same 13-byte binary header
  (`agent-wire-protocol.ts:1-13`) for the handshake *and* every subsequent
  RPC — there is no plain-JSON phase at any point in the real client.

`backend-go/services/infra-fleet-service/internal/adapter/agentwsserver/server.go:120-172`
implements exactly this — the real protocol, byte-for-byte. Changing it to
match the documented plain-JSON flow would **break the real agent that
ships with this system**, a regression, not a fix, in exchange for
interop with a specification no client (real or documented-as-an-example)
actually exercises differently.

### Why: this exact question was already litigated for the TS predecessor, and settled the other way

This is not a hypothetical judgment call — the identical divergence was
found and fixed once already, for the TS backend this system replaces:

- `specs/backend/bugs/hld-v1/BUG-BE-HLD-019-agent-ws-protocol-keepalive-closecode-version-mismatch.md`
  found the same three gaps in the TS code (`backend/src/main/dev-server/`)
  against the same source doc family (`docs/features/F29-agent-websocket-protocol.md`):
  close codes 4001/4002/4003 don't exist in the real code (which uses
  standard WS 1008/1005), keepalive timing doesn't match, and
  `AGENT_MIN_VERSION` was a dead constant.
- Its solution, `specs/backend/bugs/hld-v1/solutions/SOLUTION-agent-ws-protocol-exact.md`,
  concluded explicitly (§"Kết luận trước khi vá"): *"code thật... đã hoạt
  động ổn định và nhất quán... nên không sửa số liệu code, chỉ sửa doc cho
  khớp"* ("the real code already works, stably and consistently — don't
  change the code's numbers, fix the doc to match"), and for the one
  genuinely-missing piece (version-mismatch enforcement), implemented it
  reusing the **existing** 1008 close code with a distinguishing message
  string, explicitly rejecting inventing a custom 4000-range code
  ("Không dùng close code tùy biến (4000-4999)... client phân loại theo
  message, không theo code").

Both the source doc (`F29-agent-websocket-protocol.md`) and the redesign
doc (`BL-AWS-02-direct-websocket.md`) most likely trace back to the same
originally-aspirational design note; the TS-era investigation already
established which side of that gap was authoritative. Re-deciding it the
other way in Go, with no new evidence, would silently reopen a question
this system already has a documented, reasoned answer to.

### Corroborating evidence: the real agent doesn't rely on the close-code taxonomy either

`agent/src/relay/agent-connection-direct.ts`'s own reconnect logic (`FIX
BUG-DS-AWS` comment) treats **any** close before a successful handshake as
a token problem needing renewal, explicitly because "the exact wire code
isn't reliable across ws versions/proxies (a bare `ws.close()` with no
code surfaces as 1005, not 1008)." The real client was deliberately built
to *not* depend on distinguishing 4001 from 4002 from 1008 — further
evidence that a precise custom close-code taxonomy was never load-bearing
for interop, only for a spec doc nobody's client implements.

### What this means for Option A

`08-inter-service-communication.md`'s "Talking to the Dev Server Agent"
section frames this as a live decision between Option A (keep the
existing wire protocol, `agent/` doesn't change) and Option B (redesign to
gRPC streaming, `agent/` changes) — and states "Default recommendation:
Option A for the initial Go rewrite." This bug's fix *is* Option A,
applied literally: `agentwsserver/server.go`'s binary-first implementation
already is a faithful Go port of the existing protocol; nothing here is a
deviation from Option A that needs correcting. The deviation is entirely
in the *documentation*, in a doc this migration didn't author and that
predates the Go rewrite.

## What's a genuine gap, not doc drift

Two things BUG-AWS-02 flags are real, independent of the close-code
question above:

### 1. Version-mismatch check — missing, should be added (reusing 1008)

`AGENT_MIN_VERSION` exists on the agent side
(`agent/src/shared/agent-wire-protocol.ts:31`, `AGENT_MIN_VERSION =
'1.0.0'`) and `inboundHandshakeParams.AgentVersion`
(`server.go:53`) is already captured from the handshake — but nothing
compares it. Add, mirroring the TS fix's already-settled resolution
(`SOLUTION-agent-ws-protocol-exact.md` §B):

```go
// agentwsserver/server.go — after token validation, before acknowledgeHandshake
if params.AgentVersion != "" && isBelowMinimumVersion(params.AgentVersion, s.Cfg.MinAgentVersion) {
    s.rejectVersion(hctx, conn, req.ID, params.AgentVersion)
    return
}
```

```go
func (s *Server) rejectVersion(ctx context.Context, conn *websocket.Conn, requestID uint32, agentVersion string) {
    msg := fmt.Sprintf("Agent version %s is below the minimum supported version %s. Please update the Orca agent.", agentVersion, s.Cfg.MinAgentVersion)
    resp := devserveragent.JSONRPCResponse{
        JSONRPC: "2.0", ID: requestID,
        Error: &devserveragent.JSONRPCError{Code: handshakeFailedCode, Message: msg}, // AgentErrorCode.HandshakeFailed = -33100, not AuthFailed
    }
    if frame, err := devserveragent.EncodeJSONRPCFrame(resp, 1, 0); err == nil {
        _ = conn.Write(ctx, websocket.MessageBinary, frame)
    }
    conn.Close(websocket.StatusPolicyViolation, msg) // 1008 — same code as auth failure, message disambiguates, per the settled TS decision
}
```

`isBelowMinimumVersion` is a direct Go port of
`agent-wire-protocol.ts`'s `isAgentVersionBelowMinimum` (major.minor.patch
only, non-numeric segments fail open toward "too old"). `handshakeFailedCode
= -33100` mirrors `AgentErrorCode.HandshakeFailed` (already defined
identically in `agent-wire-protocol.ts:47`), distinct from the existing
`authFailedCode = -33101` (`server.go:31`) used for bad tokens — same
JSON-RPC error code the agent side already defines for this exact
condition, just not yet raised from the Go server.

### 2. Capacity limit — genuinely new, not present anywhere (TS, agent, or the doc's prior investigation)

Unlike the other three, BUG-AWS-02's "no capacity-limit handling (4004)"
finding has no precedent in either the TS-era bug or the real agent code —
it is a legitimate, never-built feature, not doc drift. Add a lightweight
cap, the same shape `infra-fleet-service.md` §8 already specifies for a
different resource ("Backpressure on terminal session count... TS
enforced `MAX_CONCURRENT_STREAMS = 16`... carry the same ceiling forward
as a coordination-layer check"):

```go
// agentwsserver/server.go
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    if s.Client.LiveSessionCount() >= s.Cfg.MaxConcurrentSessions {
        conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
        if err == nil {
            conn.Close(websocket.StatusPolicyViolation, "Server at capacity") // 1008, not a new 4004 — consistent with the decision above
        }
        return
    }
    // ... existing accept + handleConnection ...
}
```

`Cfg.MaxConcurrentSessions` defaults generously (e.g. 500) — this is a
circuit-breaker against runaway connection counts, not a tuned production
limit; the exact number is an operational decision out of this solution's
scope. `LiveSessionCount()` is a small new method on
`devserveragent.Client` reading `len(c.sessions)` under its existing
mutex — no new state.

## Agent (`agent/`) changes needed

**None**, by design — that is the entire point of this decision. Both
fixes above (version check, capacity check) are enforced purely
server-side (Orca is the WS server in direct-websocket mode) and both
close with the WS-standard code the real agent already treats
opaquely. If the doc's plain-JSON/4001-4004 design were pursued instead,
this would require a real `agent/` protocol change — flagged here
explicitly, per this task's instruction, precisely to make clear that
choosing the other path is not free and is the reason it isn't the one
taken.

## Test plan

- `agentwsserver/server_test.go` — a handshake with `agentVersion` below
  `Cfg.MinAgentVersion` is rejected with code 1008 and a message containing
  both versions; a handshake with no `agentVersion` field (older agents
  that predate the field) is **not** rejected (fail open on missing data,
  matching `firstNonEmpty`'s existing fallback posture at `server.go:198`);
  a handshake at or above the minimum succeeds unchanged.
- `agentwsserver/server_test.go` — with `LiveSessionCount()` stubbed at the
  cap, a new connection is closed 1008 before the handshake read even
  starts (assert no `Registry.Consume` call — the reject must be
  pre-auth, not post).
- Regression test: the existing binary-first handshake path
  (`server_test.go`'s current coverage of `server.go:120-172`) is
  unchanged — this solution is additive, not a rewrite.
- No test asserts a 4001–4004 close code anywhere — a lint-level
  `grep -RIn '400[1-4]' agentwsserver/` staying empty is the intended,
  documented state, not an oversight to fix later.

## References

- `backend-go/services/infra-fleet-service/internal/adapter/agentwsserver/server.go:17-35,95-221` — current handshake handler this solution extends, not replaces
- `agent/src/shared/agent-wire-protocol.ts:18,31,47,62-63` — `AGENT_HANDSHAKE_METHOD`, `AGENT_MIN_VERSION`, `AgentErrorCode.HandshakeFailed`, "first message after WS connected"
- `agent/src/relay/agent-connection-stdio.test.ts:123,251` — real-agent test proving binary-first, agent-speaks-first behavior
- `agent/src/relay/agent-connection-direct.ts` (`FIX BUG-DS-AWS` comment) — real client's own "close code isn't reliable" reasoning
- `specs/backend/bugs/hld-v1/BUG-BE-HLD-019-agent-ws-protocol-keepalive-closecode-version-mismatch.md` — the TS-era discovery of this exact divergence
- `specs/backend/bugs/hld-v1/solutions/SOLUTION-agent-ws-protocol-exact.md` §"Kết luận trước khi vá", §B — the settled "fix the doc, implement version-check with 1008" resolution this solution ports to Go
- `specs/backend-go/tdd/architecture/08-inter-service-communication.md:84-108` — Option A vs. B, "Default recommendation: Option A"
- `specs/backend-go/tdd/services/infra-fleet-service.md:475-478` (§8, `MAX_CONCURRENT_STREAMS` precedent for the capacity check)
- `docs/logic/agent-ws/BL-AWS-02-direct-websocket.md:36-88` — the doc this solution recommends fixing, not the code
- [SOL-AWS-03](./SOL-AWS-03-agent-token-management.md) — `RevokeAgentToken`'s live-session close reuses this solution's 1008-not-4001 decision
