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
       │  (2) Orca sends: { type: "handshake-request" }
       │  (3) Agent sends: { type: "agent.handshake", agentToken, name, version }
       │  (4) Orca validate agentToken → lookup trong DevServer config
       │  (5) Orca sends: { type: "handshake-ok", sessionId }
       │
       ↕  Binary frames + JSON-RPC
       │
SshChannelMultiplexer (Orca)
       │  via WsTransport adapter
       ↕
Agent logic
```

## Handshake Protocol

```
C→S: HTTP Upgrade (GET /agent, Connection: Upgrade, Upgrade: websocket)
S→C: { "type": "handshake-request", "version": "1" }
C→S: { "type": "agent.handshake", "agentToken": "<token>", "name": "my-agent", "version": "1.0.0" }
S→C (OK): { "type": "handshake-ok", "sessionId": "sess-abc123" }
S→C (FAIL): WS close(4001, "Invalid agent token")
```

## Agent implements WS Client (Python example)

```python
import asyncio, websockets, struct, json

async def connect_to_orca():
    uri = "ws://localhost:6768/agent"
    async with websockets.connect(uri) as ws:
        # Receive handshake-request
        msg = json.loads(await ws.recv())
        assert msg["type"] == "handshake-request"
        
        # Send handshake
        await ws.send(json.dumps({
            "type": "agent.handshake",
            "agentToken": "my-secret-token",
            "name": "my-python-agent",
            "version": "1.0.0"
        }))
        
        # Receive handshake-ok
        ok = json.loads(await ws.recv())
        assert ok["type"] == "handshake-ok"
        
        # Now send binary frames with JSON-RPC
        await send_frame(ws, build_rpc_frame("worktree.list", {}))
```

## Go example

```go
conn, _, err := websocket.DefaultDialer.Dial("ws://localhost:6768/agent", nil)
// ... handshake flow ...
```

## Error Codes

| Close Code | Reason |
|-----------|--------|
| 4001 | Invalid or missing agentToken |
| 4002 | Handshake timeout (30s) |
| 4003 | Protocol version mismatch |
| 4004 | Server at capacity |

## Source References

- `src/main/agent-ws/agent-ws-server.ts` — AgentWebSocketServer
- `src/main/agent-ws/ws-transport.ts` — WsTransport
- `src/main/server/web-server.ts` — mount /agent endpoint
