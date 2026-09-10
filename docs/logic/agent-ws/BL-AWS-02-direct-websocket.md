# BL-AWS-02: direct-websocket Mode (Agent → Orca)

**Domain:** Agent WebSocket  
**Priority:** P1  
**Actor chính:** Agent Developer  
**Tham chiếu:** FR-17.2, UR-130, F29

---

## Mô tả

Trong direct-websocket mode, Agent (client) kết nối tới WebSocket server của Orca. Agent đóng vai WS client, Orca đóng vai WS server tại endpoint `/agent`.

## Architecture

```
Custom AI Agent (Python/Go/TypeScript)
       │
       │  (1) HTTP Upgrade: ws://orca:6768/agent
       ▼
AgentWebSocketServer (Orca)
       │
       │  (2) Agent sends first: agent.handshake (13-byte-framed JSON-RPC —
       │      no preceding server push, see "Handshake Protocol" below)
       │  (3) Orca validates agentToken → Registry.Consume, then checks
       │      agentVersion against MinAgentVersion
       │  (4) Orca replies: {ok, orcaVersion, sessionId} (JSON-RPC result)
       │
       ↕  Binary frames + JSON-RPC
       │
SshChannelMultiplexer (Orca)
       │  via WsTransport adapter
       ↕
Agent logic
```

> **Correction (SOL-AWS-02):** the sequence and close-code table below were
> rewritten to match what `agentwsserver/server.go` and
> `agent/src/relay/agent-connection-direct.ts` actually implement — see
> `specs/backend-go/bugs/logic-v1/solutions/SOL-AWS-02-direct-websocket-protocol-divergence.md`.
> This exact divergence/resolution was already litigated once for the TS
> backend — see
> `specs/backend/bugs/hld-v1/solutions/SOLUTION-agent-ws-protocol-exact.md`.
> The previous version of this section documented a plain-JSON,
> server-initiated push/ack exchange and a table of four non-standard
> WS close codes in the 4000 range that neither side of this codebase has
> ever implemented.

## Handshake Protocol

The exchange is **binary-first and client-initiated**: the agent sends
`agent.handshake` as its very first message once the WS connects — Orca
never pushes anything first. Every frame (in both directions) is
13-byte-framed JSON-RPC (Stack B, see `devserveragent/frame.go`), not plain
JSON text.

```
C→S: HTTP Upgrade (GET /agent, Connection: Upgrade, Upgrade: websocket)
C→S: [13-byte frame header][JSON-RPC request] { "method": "agent.handshake",
       "params": { "agentToken": "<token>", "platform", "arch", "nodeVersion",
                    "agentVersion", "capabilities" } }
S→C (OK): [13-byte frame header][JSON-RPC result]
       { "ok": true, "orcaVersion": "<version>", "sessionId": "sess-..." }
S→C (FAIL): [13-byte frame header][JSON-RPC error] then WS close 1008
```

See `backend-go/services/infra-fleet-service/internal/adapter/agentwsserver/server.go:120-172`
(`handleConnection`/`acknowledgeHandshake`) and
`agent/src/shared/agent-wire-protocol.ts:62-63` for the exact frame/result
shapes.

### Close conditions

Both rejection paths use standard WS code **1008** (Policy Violation) —
disambiguated by the JSON-RPC error message text, not by a distinct close
code:

- **Auth failure** — invalid or unregistered `agentToken`. JSON-RPC error
  code `-33101` (`AgentErrorCode.AuthFailed`), message
  `"Authentication failed: invalid or unregistered agent token"`. See
  `server.go`'s `rejectHandshake`.
- **Version mismatch** — `agentVersion` below the configured
  `MinAgentVersion` (added by TASK-AWS-02-01/02). JSON-RPC error code
  `-33100` (`AgentErrorCode.HandshakeFailed`), message names both the
  agent's reported version and the minimum. See `server.go`'s
  `rejectVersion`. A handshake with no `agentVersion` field at all is
  **not** rejected — fails open toward agent builds too old to send the
  field.
- **Capacity limit** (TASK-AWS-02-03) — also close 1008, message
  `"Server at capacity"`, sent before the handshake frame is even read
  (pre-auth).

No custom 4000-range close code is used anywhere in this exchange —
custom close codes aren't reliably preserved across `ws` library versions
and intermediating proxies, so every rejection path collapses to standard
1008 disambiguated by JSON-RPC error message text instead. See
`agent/src/relay/agent-connection-direct.ts`'s `FIX BUG-DS-AWS` comment:
the agent already treats *any* pre-handshake close as a token problem
rather than branching on the wire close code, precisely because the code
isn't trustworthy transport-to-transport (a bare `ws.close()` with no code
can surface as 1005, not 1008).

## Agent implements WS Client (Python example)

```python
import asyncio, websockets, json

async def connect_to_orca():
    uri = "ws://localhost:6768/agent"
    async with websockets.connect(uri) as ws:
        # First message is agent.handshake — no preceding server push.
        await ws.send(build_rpc_frame("agent.handshake", {
            "agentToken": "my-secret-token",
            "platform": "linux",
            "arch": "x64",
            "nodeVersion": "20.11.0",
            "agentVersion": "1.0.0",
        }))

        # Receive the {ok, orcaVersion, sessionId} result frame.
        frame = await ws.recv()
        result = decode_rpc_frame(frame)
        assert result["ok"]

        # Now send further binary frames with JSON-RPC.
        await ws.send(build_rpc_frame("worktree.list", {}))
```

## Go example

```go
conn, _, err := websocket.DefaultDialer.Dial("ws://localhost:6768/agent", nil)
// conn.WriteMessage(websocket.BinaryMessage, encodeFrame("agent.handshake", params))
// ... read the {ok, orcaVersion, sessionId} result frame ...
```

## Source References

- `backend-go/services/infra-fleet-service/internal/adapter/agentwsserver/server.go` — `Server.handleConnection`/`rejectHandshake`/`rejectVersion`/`acknowledgeHandshake`
- `backend-go/services/infra-fleet-service/internal/adapter/agentwsserver/capacity.go` — `MaxConcurrentSessions` circuit breaker
- `agent/src/relay/agent-connection-direct.ts` — agent-side direct-websocket client, incl. the `FIX BUG-DS-AWS` close-handling note
- `agent/src/shared/agent-wire-protocol.ts` — `AGENT_MIN_VERSION`, `AgentErrorCode`, frame/JSON-RPC shapes
