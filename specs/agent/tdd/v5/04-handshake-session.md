# TDD-AG-04: Handshake & Session Management

**Document:** TDD-AG-04
**Version:** 2.0
**Date:** 2026-07-28
**Domain:** Handshake protocol, session lifecycle, lastConnHandshakeOk tracking
**Source:** `src/relay/agent-session.ts` — `sendHandshake()`, `createSession()` in agent-session.ts
**HLD Ref:** C3.8
**ADR:** ADR-005

---

## 1. Handshake Request (Agent → Orca)

```javascript
function sendHandshake(ws, token) {
  const rpc = {
    jsonrpc: '2.0',
    id: 1,
    method: 'agent.handshake',
    params: {
      agentToken:   token,        // AGENT_TOKEN env var
      devServerId:  DEV_ID,       // DEV_SERVER_ID env var
      capabilities: ['rpc', 'tools'],
      tools:        discoveredTools.map(t => t.name), // e.g. ['claude_code', 'git', 'gh', ...]
      version:      '1.0.0',
    },
  };
  ws.send(encodeFrame(FRAME_TYPE.HANDSHAKE, JSON.stringify(rpc)));
}
```

**Sent on `ws.on('open')`** — immediately after WebSocket connects.

---

## 2. Handshake Response (Orca → Agent)

```json
// Success:
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "status": "ok",
    "sessionId": "sess-xxxxxxxx"
  }
}

// Failure (token rejected/expired):
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": 1008,
    "message": "Invalid or expired agent token"
  }
}
```

---

## 3. lastConnHandshakeOk Flag

```javascript
let lastConnHandshakeOk = false;

// In handleSession():
if (rpc.result?.status === 'ok') {
  handshakeDone = true;
  lastConnHandshakeOk = true;
  log.info(`Handshake OK: sessionId=${rpc.result.sessionId}`);
} else if (rpc.error) {
  log.error(`Handshake failed: ${JSON.stringify(rpc.error)}`);
  ws.close(1008, 'Handshake failed');
}

// In connectDirect() close handler:
if (lastConnHandshakeOk) {
  log.warn(`Connection dropped after handshake → exit(2) for fresh token restart`);
} else {
  log.error(`Connection closed before handshake → token rejected/expired → exit(2)`);
}
```

**Mục đích:** Distinguish giữa:
- Pre-handshake close: token bị reject → log error
- Post-handshake close: network drop → log warn

Cả hai đều exit(2) để systemd restart với token mới.

---

## 4. Session Lifecycle

| Phase | Description |
|-------|-------------|
| Pre-connect | `seqCounter=0`, `highestAck=0` reset |
| ws.open | `sendHandshake()` + `startKeepalive()` |
| Handshaking | Only process response với `id=1` (handshake reply) |
| Session active | `handshakeDone=true` → `dispatchRpc()` for all subsequent frames |
| ws.close code=1000 | Clean close → `exit(0)` |
| ws.close code≠1000 | Unexpected → `exit(2)` after 200ms delay |
| SIGTERM | `exit(0)` |

---

## 5. Handshake Timeout (Server Side)

Orca Server: `AgentWebSocketServer` có handshake timeout 20s (ADR-005).
- Nếu agent không gửi `agent.handshake` trong 20s → server closes connection
- Frame TYPE phải = `0x01` (Regular) → non-Regular frames SILENTLY IGNORED → timeout

---

## 6. Tools Advertisement trong Handshake

Agent gửi danh sách tools hiện có trong handshake:
```javascript
tools: discoveredTools.map(t => t.name)
// Ví dụ: ['claude_code', 'claude_code_file', 'gh', 'git', 'gitnexus', 'codegraph', 'docker', 'shell', 'read_file', 'list_dir']
```

Orca Server lưu danh sách này vào `DevServerManager` → UI hiển thị tools available cho dev server.

Khi server gọi `tools/list` → agent trả lại danh sách đầy đủ với `inputSchema`.
