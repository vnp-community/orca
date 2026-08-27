# TASK-AWS-02-04: Fix `BL-AWS-02-direct-websocket.md` to match the real binary-first protocol

**From Solution:** SOL-AWS-02
**Priority:** P2
**Service:** n/a (documentation — no backend-go code)
**File:** `docs/logic/agent-ws/BL-AWS-02-direct-websocket.md`
**Depends on:** TASK-AWS-02-02, TASK-AWS-02-03
**Status:** `[ ]` TODO

---

## Context

`docs/logic/agent-ws/BL-AWS-02-direct-websocket.md:36-88` documents a
plain-JSON `handshake-request`/`agent.handshake`/`handshake-ok` exchange
and a 4001–4004 close-code table that neither `agentwsserver/server.go`
nor the real `agent/` client (`agent/src/relay/agent-connection-direct.ts`,
`agent-wire-protocol.ts`) has ever implemented. SOL-AWS-02's decision is to
fix the doc, not the code — this is the required companion PR flagged in
that solution's "Affected files" list. This is the one task in this batch
that is documentation-only; it exists so the doc doesn't keep contradicting
the (correct, now-shipping) code after TASK-AWS-02-01..03 land.

## Changes to make

In `docs/logic/agent-ws/BL-AWS-02-direct-websocket.md`, replace the
plain-JSON handshake sequence and the 4001–4004 close-code table
(`:36-88`) with:

1. The real binary-first exchange: the agent sends `agent.handshake`
   (13-byte-framed JSON-RPC, no preceding `handshake-request` push) as its
   very first message once the WS connects; Orca responds with a
   `handshake-ok`-shaped JSON-RPC result (`{ok, orcaVersion, sessionId}`)
   over the same binary frame format — cite
   `backend-go/services/infra-fleet-service/internal/adapter/agentwsserver/server.go:120-172`
   and `agent/src/shared/agent-wire-protocol.ts:62-63`.
2. Two close conditions, both using standard WS code **1008** (Policy
   Violation), disambiguated by the JSON-RPC error message text, not by
   close code: auth failure (`authFailedCode = -33100`... — confirm the
   exact code from `server.go` at write time) and version mismatch
   (`handshakeFailedCode`, added by TASK-AWS-02-02). Remove the 4001–4004
   table entirely; add a short note explaining why (custom close codes
   aren't reliable across ws versions/proxies — cite
   `agent/src/relay/agent-connection-direct.ts`'s `FIX BUG-DS-AWS` comment).
3. The capacity-limit behavior added by TASK-AWS-02-03 (also 1008, message
   `"Server at capacity"`).

Cross-reference `specs/backend-go/bugs/logic-v1/solutions/SOL-AWS-02-direct-websocket-protocol-divergence.md`
at the top of the rewritten section as the source of this correction, and
note (as that solution does) that this same divergence/resolution was
already litigated once for the TS backend — see
`specs/backend/bugs/hld-v1/solutions/SOLUTION-agent-ws-protocol-exact.md`.

## Verify

```bash
cd /opt/repos/orca
grep -n "handshake-request\|400[1-4]" docs/logic/agent-ws/BL-AWS-02-direct-websocket.md
```

Expected: no remaining references to `handshake-request` or a 4001–4004
close code — the doc now describes exactly what
`agentwsserver/server.go` and `agent/src/relay/agent-connection-direct.ts`
actually do.
