# BUG-DS-001 — relay-websocket Handshake Token Không Khớp

**ID:** BUG-DS-001  
**Mức độ:** 🔴 Critical  
**Module:** relay-websocket handshake  
**Phát hiện:** 2026-07-26  
**Status:** 🔴 Open

---

## Mô Tả

relay-websocket mode hoàn toàn không kết nối được. Orca connect vào agent thành công ở TCP level nhưng handshake bị reject ngay lập tức do token validation logic trong agent.js sai.

---

## Root Cause

**Orca (server side)** — `dev-server-relay-bridge.ts` L277-285:

```typescript
// Strip token từ URL, gửi qua Authorization header
const token = url.searchParams.get('token') ?? ''
url.searchParams.delete('token')
const cleanUrl = url.toString()

const ws = new WebSocket(cleanUrl, {
  headers: token ? { Authorization: `Bearer ${token}` } : {},
  // ← token trong HTTP header, KHÔNG trong handshake params
})
```

**`runOrcaInitiatorHandshake`** — `ws-handshake.ts` gửi:
```json
{
  "method": "agent.handshake",
  "params": {
    "orcaVersion": "1.4.138"
    // ← KHÔNG có "agentToken" field
  }
}
```

**agent.js** (relay-ws receiver path):
```javascript
case rpc.method === 'agent.handshake':
  const incoming = rpc.params?.agentToken;  // ← undefined!
  if (incoming !== relayToken) {            // undefined !== 'relay-secret'
    ws.close(1008, 'Unauthorized');         // ← REJECT!
    return;
  }
```

`rpc.params.agentToken` luôn là `undefined` vì Orca không gửi field này trong handshake params. So sánh `undefined !== 'relay-secret'` → true → reject.

---

## Tái Hiện

1. Start agent relay-ws: `MODE=relay-websocket AGENT_TOKEN=relay-secret node agent.js`
2. Trong Orca UI: Add Dev Server → relay-websocket, URL `ws://172.20.2.31:6799/orca-relay?token=relay-secret`
3. Click "Test Connection"

**Kết quả**: `Handshake rejected (bad token from Orca)` trong agent logs.

---

## Hậu Quả

- relay-websocket mode **không hoạt động** từ đầu đến cuối
- Mọi dev server cấu hình dạng relay-ws đều fail ở bước handshake
- User không có thông báo rõ ràng — chỉ thấy "Connection failed"

---

## Fix

**Phương án A — Bỏ token re-validation trong handshake** (đơn giản nhất):

Token đã được validate ở `wss.on('connection')` qua query param/Authorization header. Không cần validate lại trong handshake frame.

```javascript
// agent.js — handleSession relay-ws path
if (rpc.method === 'agent.handshake') {
  // Token đã được validate tại wss.on('connection') — không re-validate ở đây
  const okResp = {
    jsonrpc: '2.0', id: rpc.id,
    result: {
      ok: true, devServerId: DEV_ID,
      platform: process.platform, arch: process.arch,
      nodeVersion: process.version, agentVersion: '1.0.0',
      sessionId: `sess-${Date.now()}`,
      tools: discoveredTools.map(t => t.name),
    },
  };
  ws.send(encodeFrame(FRAME_TYPE.HANDSHAKE_OK, JSON.stringify(okResp)));
  handshakeOk = true;
  keepalive = startKeepalive(ws);
  log.info(`Handshake OK [relay-ws]`);
}
```

**Phương án B — Server gửi token trong handshake params** (đồng bộ với spec):

Cập nhật `runOrcaInitiatorHandshake` để include `agentToken` trong params. Nhưng server không có token sau khi strip từ URL → cần pass token vào hàm.

---

## Files Liên Quan

| File | Vị trí | Vai trò |
|------|--------|---------|
| `deploy/dev/agent/agent.js` | handleSession relay-ws path | Bug location |
| `src/main/dev-server/ws-handshake.ts` | `runOrcaInitiatorHandshake` | Không gửi agentToken |
| `src/main/dev-server/dev-server-relay-bridge.ts` | L277-285 | Strip token, không forward |
