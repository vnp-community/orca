# BL-AWS-01: relay-websocket Mode (Orca → Agent)

**Domain:** Agent WebSocket  
**Priority:** P1  
**Actor chính:** Agent Developer  
**Tham chiếu:** FR-17.1, UR-130, F29

---

## Mô tả

Trong relay-websocket mode, Orca (client) kết nối tới một WebSocket server do Agent chạy. Agent đóng vai WS server. Phù hợp cho agents chạy trong environment của agent (e.g., trên Dev Server).

## Architecture

```
Orca Desktop/Web Server
       │
       │  (1) Đọc relayWebsocketUrl từ DevServer config
       │  (2) HTTP Upgrade: ws://agent:6799/orca-relay
       │       Header: Authorization: Bearer <agentToken>
       ▼
Agent WebSocket Server (agent's process)
       │
       │  (3) Validate Bearer token
       │  (4) Accept upgrade → WS connection established
       │
       ↕  Binary frames + JSON-RPC
       │
SshChannelMultiplexer (Orca side)
       │  via WsTransport adapter
       ↕
Agent logic (full JSON-RPC access)
```

## Configuration

DevServer record phải có `relayWebsocketUrl`:
```json
{
  "id": "ds-abc",
  "hostname": "dev1.example.com",
  "relayWebsocketUrl": "ws://dev1.example.com:6799/orca-relay",
  "agentToken": "<sha256-hashed-token>"
}
```

## Agent implements WS Server (TypeScript example)

```typescript
import { WebSocketServer } from 'ws';

const wss = new WebSocketServer({ port: 6799, path: '/orca-relay' });

wss.on('connection', (ws, req) => {
  const token = req.headers.authorization?.split(' ')[1];
  if (!validateToken(token)) {
    ws.close(4001, 'Unauthorized');
    return;
  }
  // Handle binary frames per protocol spec
  ws.on('message', (data: Buffer) => {
    const header = parseHeader(data); // 13-byte header
    const payload = JSON.parse(data.slice(13).toString('utf8'));
    handleJsonRpc(payload, ws);
  });
});
```

## Wire Protocol Reference

```
Frame = [TYPE: 1B][SEQ: 4B BE][ACK: 4B BE][LEN: 4B BE][PAYLOAD: LEN bytes]

TYPE values:
  0x00 = DATA
  0x01 = ACK
  0x02 = KEEPALIVE (30s interval)
  0x03 = CLOSE
```

## Source References

- `src/main/agent-ws/ws-transport.ts` — WsTransport adapter
- `src/main/agent-ws/relay-ws-client.ts` — Orca WS client logic
- `src/main/dev-server/dev-server-manager.ts` — relayWebsocketUrl field
