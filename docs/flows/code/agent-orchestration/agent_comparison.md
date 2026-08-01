# So Sánh: Spec Solutions vs. deploy/dev/agent

> **Specs**: [specs/backend/crs/v2/agent/solutions/](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/specs/backend/crs/v2/agent/solutions/)  
> **Thực tế**: [deploy/dev/agent/agent.js](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/deploy/dev/agent/agent.js)

---

## 1. Tổng quan

| Tiêu chí | Spec (SOL-AG-001~004) | deploy/dev/agent.js |
|----------|----------------------|---------------------|
| **Ngôn ngữ** | TypeScript (server-side) | JavaScript (Node.js, CommonJS) |
| **Mục tiêu** | Server-side: ws-transport, handshake, AgentWsServer, RelayBridge | Client-side: agent chạy trên dev server |
| **Scope** | Orca Server nhận kết nối từ agent | Agent khởi tạo kết nối đến Orca Server |
| **Test coverage** | 66/66 unit tests | Không có unit tests |
| **Kiến trúc** | Module hóa rõ ràng, TypeScript types | Monolithic single-file |

> **Quan trọng**: Đây là **hai phía của cùng kết nối**. Spec định nghĩa phía *Orca server*, agent.js là phía *client agent*. Chúng phải tuân thủ cùng một wire protocol.

---

## 2. Wire Protocol — SOL-AG-001 vs. agent.js

### ✅ Phần đồng thuận

| Protocol element | Spec | agent.js |
|-----------------|------|----------|
| Frame header | `[TYPE u8][SEQ u32 BE][ACK u32 BE][LEN u32 BE][PAYLOAD]` | Giống hệt — 13 bytes |
| `MessageType.Regular` | `0x01` | `FRAME_TYPE.DATA = 0x01` ✅ |
| `MessageType.KeepAlive` | `0x09` | `FRAME_TYPE.PING = 0x09` ✅ (sau fix) |
| Handshake method | `agent.handshake` | `method: 'agent.handshake'` ✅ |
| `agentToken` field | `params.agentToken` | `params.agentToken` ✅ |

### ⚠️ Phần lệch (sau khi đã fix)

| Element | Spec | agent.js (ban đầu) | Trạng thái |
|---------|----- |--------------------|-----------|
| HANDSHAKE frame type | `0x01` (Regular) | `0x10` (sai!) | ✅ Đã fix → `0x01` |
| HANDSHAKE_OK type | `0x01` (Regular) | `0x11` (sai!) | ✅ Đã fix → `0x01` |
| PING type | `0x09` (KeepAlive) | `0x20` (sai!) | ✅ Đã fix → `0x09` |

> [!CAUTION]
> **Bug nghiêm trọng nhất của toàn session**: agent.js tự định nghĩa FRAME_TYPE với các giá trị tùy ý (`0x10`, `0x11`, `0x20`) không khớp với relay-protocol.ts trên server (`0x01`, `0x09`). Server's `FrameDecoder` **silently ignore** tất cả frame type ≠ 1, gây 20s handshake timeout mỗi lần.

---

## 3. Handshake Flow — SOL-AG-002 vs. agent.js

### Spec (Orca server nhận handshake từ agent):

```
[Server] runOrcaReceiverHandshake():
  1. Start 20s timer
  2. ws.on('message') → FrameDecoder.feed(data)
  3. FrameDecoder → frame.type === Regular(1) → parse JSON-RPC
  4. msg.method === 'agent.handshake' → validateToken(agentToken)
  5. Token valid → send handshake_ok response
  6. Resolve với WsHandshakeInfo
```

### agent.js (client gửi handshake đến server):

```javascript
// sendHandshake():
ws.send(encodeFrame(FRAME_TYPE.HANDSHAKE, JSON.stringify({
  jsonrpc: '2.0', id: 1,
  method: 'agent.handshake',
  params: { agentToken, devServerId, capabilities, tools, version }
})));

// ws.on('message') → handleSession():
case FRAME_TYPE.HANDSHAKE_OK: // = 0x01 (sau fix)
  if (rpc.result?.ok) handshakeOk = true;
```

### Vấn đề còn lại trong agent.js:

| Issue | Mô tả | Mức độ |
|-------|-------|--------|
| **Switch-case collision** | Tất cả `FRAME_TYPE.*` đều = `0x01` → switch `case HANDSHAKE:`, `case HANDSHAKE_OK:`, `case DATA:` **đều map về case `0x01`** — chỉ case đầu tiên được execute | 🔴 Nghiêm trọng |
| **Keepalive type conflict** | `PING = PONG = 0x09` nhưng switch có `case FRAME_TYPE.PING:` và `case FRAME_TYPE.PONG:` riêng — JavaScript chạy đầu tiên gặp | 🟡 Minor |
| **ACK không dùng** | agent gửi `ack=0` luôn, không track ack từ server | 🟢 Low |

> [!WARNING]
> **Switch-case với enum=0x01**: Sau khi fix FRAME_TYPE, tất cả các case `HANDSHAKE`, `HANDSHAKE_OK`, `DATA` đều bằng `0x01`. JavaScript switch sẽ chỉ match case đầu tiên (`HANDSHAKE`). Điều này có thể gây ra DATA frames bị xử lý như HANDSHAKE!

---

## 4. Session Management — SOL-AG-004 vs. agent.js

### Spec (server-side):

```typescript
// DevServerRelayBridge.connectWithExternalToken():
registerSlot(token, (mux, info) => {
  this.session = mux
  mux.onDispose(() => { this.session = null })  // ← clear on disconnect
  resolve(info)
})
```

### agent.js (client-side):

```javascript
// handleSession():
function handleSession(ws, isInitiator, relayToken) {
  let handshakeOk = false;
  lastConnHandshakeOk = false;  // module-level flag
  // ...
}

// connectDirect():
ws.on('close', code => {
  if (code === 1000) process.exit(0);
  // Always exit(2) → systemd restart → fresh token
  setTimeout(() => process.exit(2), 200);
})
```

### So sánh:

| Aspect | Spec | agent.js |
|--------|------|----------|
| **Reconnect strategy** | Server quản lý session, agent có thể reconnect | Agent exit(2) → systemd restart → token mới |
| **Session state** | `bridge.session` cleared via `onDispose` | Module-level `lastConnHandshakeOk` flag |
| **Token lifecycle** | Server slot: 60s timeout → onExpired | Agent: token là one-time, consumed on connect |
| **Daemon mode** | N/A (server-side) | systemd service với `Restart=always` |

---

## 5. relay-websocket Mode — SOL-AG-003 vs. agent.js

### Spec (Orca connect đến agent's WS server):

```typescript
// runOrcaInitiatorHandshake():
ws.send(frame({ method: 'agent.handshake', params: { orcaVersion } }))
ws.on('message') → expect agent.handshake response với { ok: true }
```

### agent.js (agent listen, Orca connects):

```javascript
// listenRelay():
const wss = new WebSocketServer({ port: AGENT_PORT, path: '/orca-relay' })
wss.on('connection', (ws, req) => {
  // Token auth via query param or Authorization header
  handleSession(ws, false, token)  // isInitiator=false
})

// handleSession relay path:
case FRAME_TYPE.HANDSHAKE:  // = 0x01 (sau fix)
  // Validate relayToken
  ws.send(encodeFrame(FRAME_TYPE.HANDSHAKE_OK, JSON.stringify(ok)))
```

### Vấn đề:

| Issue | Mô tả |
|-------|-------|
| **Token auth khác spec** | Spec: `params.agentToken` trong HANDSHAKE frame. agent.js: query param `?token=` hoặc `Authorization` header |
| **Path không khớp** | Spec không định nghĩa path cho relay-ws. agent.js dùng `/orca-relay` |

---

## 6. Điều agent.js có mà Spec không có

| Feature | Mô tả |
|---------|-------|
| **Tool Discovery** | Tự detect claude, gh, git, docker... khi startup |
| **Tool Registry** | 8 tools với handler: claude_code, shell, docker, git, gitnexus, codegraph, read_file, list_dir |
| **MCP tools/call** | Implement `tools/list` và `tools/call` JSON-RPC methods |
| **Process spawning** | `spawn()` với streaming stdout/stderr cho long-running commands |
| **TOOL_PATH setup** | PATH management để tìm CLIs trong `~/.local/bin` |
| **Daemon lifecycle** | `lastConnHandshakeOk`, `exit(2)` logic cho systemd |

---

## 7. Điều Spec có mà agent.js thiếu hoặc sai

| Spec requirement | Trạng thái agent.js |
|-----------------|---------------------|
| Frame type phải đúng với relay-protocol | ✅ Đã fix |
| `devServerId` trong handshake params | ✅ Có (`DEV_ID`) |
| `platform`, `arch`, `nodeVersion` trong handshake | ⚠️ Thiếu `arch`, `platform` — chỉ có `version` |
| `agentVersion` field | ⚠️ Gửi `version` thay vì `agentVersion` |
| Keepalive 5s interval | ⚠️ Agent dùng 30s mặc định (KEEPALIVE_SEND_MS = 5_000 trong spec) |
| ACK tracking | ❌ Không track ACK (luôn gửi ack=0) |

---

## 8. Khuyến nghị Fix

### Ưu tiên cao:

```javascript
// FIX 1: Tách frame type riêng cho DATA vs HANDSHAKE vs HANDSHAKE_OK
// Vì sau khi fix, switch-case không phân biệt được HANDSHAKE vs DATA vs HANDSHAKE_OK

// Giải pháp: dùng JSON-RPC method để phân biệt, không dùng frame type
// Tất cả frames đều type=0x01, phân biệt bằng msg.method hoặc msg.result/msg.id

ws.on('message', (data, isBinary) => {
  const frame = decodeFrame(data);
  if (frame.type === 0x09) { /* keepalive */ return; }
  if (frame.type !== 0x01) { return; }  // ignore unknown
  
  const msg = JSON.parse(frame.payload.toString('utf8'));
  
  if (!handshakeOk) {
    // Expect handshake_ok response (result.ok=true)
    if (msg.result?.ok) { handshakeOk = true; lastConnHandshakeOk = true; }
    else if (msg.error) { /* auth failed */ process.exit(2); }
    return;
  }
  // After handshake: dispatch RPC
  dispatchRpc(ws, frame.payload.toString('utf8'));
})
```

### Ưu tiên thấp:

```javascript
// FIX 2: Keepalive interval theo spec (5s thay vì 30s)
function startKeepalive(ws, ms = 5000) { ... }

// FIX 3: Thêm platform, arch, agentVersion vào handshake params
params: {
  agentToken, devServerId,
  agentVersion: '1.0.0',
  platform: process.platform,  // ← thêm
  arch: process.arch,           // ← thêm
  nodeVersion: process.version,
  capabilities: ['rpc', 'tools'],
  tools: discoveredTools.map(t => t.name),
}
```

---

## 9. Kết luận

```
Spec (SOL-AG-001~004)           deploy/dev/agent.js
─────────────────────           ────────────────────
Server-side TypeScript      ←→  Client-side JavaScript
Modular architecture        ←→  Monolithic single file  
Type-safe, tested            ←→  Runtime-only, no tests
Protocol receiver            ←→  Protocol initiator
```

**Alignment hiện tại**: ~75% sau các fixes.  
**Bug cốt lõi đã fix**: Frame type mismatch (0x10 → 0x01).  
**Remaining gap**: Switch-case ambiguity cần refactor; keepalive interval; handshake params thiếu `platform`/`arch`.
