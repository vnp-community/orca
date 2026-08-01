# ADR-005 — Agent WebSocket Binary Wire Protocol (13-byte Header)

| Trường | Giá trị |
|--------|---------|
| **ID** | ADR-005 |
| **Trạng thái** | 🔄 Superseded by [ADR-014](./ADR-014-gateway-agent-json-rpc-protocol.md) |
| **Ngày** | 2026-07-28 |
| **Cập nhật** | 2026-07-30 (v6.0 superseded) |
| **HLD Ref** | C3.8, C4.5 |
| **Code Ref** | `src/main/dev-server/agent-ws-server.ts`, `src/relay/protocol.ts`, `src/main/runtime/rpc/ws-transport.ts`, `deploy/dev/agent/agent.js` |

---

> **⚠️ SUPERSEDED (v6.0 — 2026-07-30)**  
> ADR-005 bị thay thế bởi **[ADR-014 — Gateway–Agent JSON-RPC Protocol v3](./ADR-014-gateway-agent-json-rpc-protocol.md)**.  
> Protocol 13-byte binary header được thay bằng JSON-RPC 2.0 over WebSocket với signed context. Lý do: ADR-014 cung cấp debuggability tốt hơn, context propagation rõ ràng hơn, và phù hợp với mô hình Gateway↔Agent (không còn AI↔Orca).  
> ADR-005 vẫn giữ lại để document lịch sử kỹ thuật.

---

## Bối cảnh

AI agents (TypeScript, Python, Go, Java...) cần giao tiếp với Orca để:
- Stream PTY output (binary, real-time)
- Gửi JSON-RPC tool calls
- Nhận ACK / flow control
- Handle reconnect sau network drop

Nếu dùng plain JSON WebSocket text frames:
- Không có sequencing → không detect lost frames
- Không có flow control → buffer bloat khi PTY output burst
- Không có keepalive tracking → silent disconnects không detected

---

## Quyết định

### Wire Protocol: 13-byte Binary Header

```
[TYPE: 1 byte][SEQ: 4 bytes BE][ACK: 4 bytes BE][LEN: 4 bytes BE]
[PAYLOAD: LEN bytes UTF-8 JSON-RPC]
```

**Frame types:**
```typescript
// Từ deploy/dev/agent/agent.js và src/relay/protocol.ts
const MessageType = {
  DATA:      0,   // JSON-RPC request/response/notification
  ACK:       1,   // Explicit ACK (SEQ number being acknowledged)
  KEEPALIVE: 2,   // Heartbeat (no payload)
  CLOSE:     3,   // Graceful shutdown
}
```

**SEQ/ACK fields:**
- `SEQ`: sender's sequence number (monotonic u32, wraps at 2^32)
- `ACK`: receiver's last seen SEQ (enables sender to know what was received)
- Enables loss detection mà không cần TCP-level sequence tracking

### 2 Connection Modes

**Mode 1: relay-websocket** (Orca → Agent WS Server)
```
Agent hosts WS server on port 6799
Orca connects: ws://agent:6799/orca-relay
Auth: Bearer token in Authorization header
Orca = initiator, runs OrcaInitiatorHandshake
```

**Mode 2: direct-websocket** (Agent → Orca WS Server)
```
Agent connects: wss://orca:6768/agent
Auth: agentToken in handshake frame
AgentWebSocketServer intercepts /agent path on HTTP server upgrade event
Orca = receiver, runs OrcaReceiverHandshake
```

### Handshake (cả 2 modes)

```json
// Agent → Orca
{ "type": "agent.handshake", "agentToken": "agt-xxx", "name": "claude-agent", "version": "1.0.0" }
// Orca → Agent (on success)
{ "type": "handshake-ok", "sessionId": "uuid-xxx" }
// Orca → Agent (on failure)
{ "type": "handshake-error", "reason": "invalid token" }
```

Handshake timeout: `AGENT_CONNECT_TIMEOUT_MS` (từ `agent-wire-protocol.ts`)

### AgentWebSocketServer

```typescript
// src/main/dev-server/agent-ws-server.ts
// Intercepts only WS upgrades to path AGENT_WS_PATH ('/agent')
// All other paths → pass through to existing OrcaRuntimeRpcServer
class AgentWebSocketServer {
  onUpgrade(req, socket, head): void  // Called from HTTP server upgrade event
  expectAgent(callback: AgentConnectionCallback, timeout): PendingSlot
  // PendingSlot expires after AGENT_CONNECT_TIMEOUT_MS
}
```

### SshChannelMultiplexer

```typescript
// src/main/ssh/ssh-channel-multiplexer.ts
// Wraps both SSH channels and WebSocket connections behind same interface
// → same WsTransport works for relay-ssh and relay-websocket
class SshChannelMultiplexer {
  createChannel(id: string): MultiplexedChannel
  // Frame encode/decode using 13-byte header protocol
}
```

---

## Lý do chọn

| Lựa chọn | Đánh giá |
|----------|---------|
| **13-byte binary header + JSON-RPC payload** ✅ | Language-agnostic; SEQ/ACK; typed frames; compact |
| Plain JSON text frames | No sequencing; no flow control; no frame types |
| WebRTC DataChannel | Too complex, browser-only conception |
| gRPC over WebSocket | Requires protobuf; setup overhead |
| NDJSON | No sequencing; no binary efficiency |
| msgpack | Better perf but less debuggable |

---

## Hậu quả

**Tích cực:**
- SDK guide có thể viết cho bất kỳ ngôn ngữ nào (protocol đơn giản)
- `agent.js` implement từ đầu trong ~100 lines (CommonJS)
- ACK cho phép sender detect loss mà không cần TCP sequence
- `KEEPALIVE` frame giải quyết AWS ALB 60s idle timeout

**Tiêu cực:**
- Custom protocol → mỗi SDK (Python, Go, Java) phải implement 13-byte encoder/decoder
- Big-endian u32 → cần test carefully trên little-endian systems
- SEQ overflow ở 2^32 — cần handle wrap-around (currently not handled)

---

## Trạng thái Implementation

✅ Protocol encoder/decoder (`relay/protocol.ts`)  
✅ AgentWebSocketServer (direct-websocket mode)  
✅ WsTransport adapter  
✅ Handshake (initiator + receiver)  
✅ `agent.js` implement protocol (deploy/dev/agent/)  
✅ KEEPALIVE frame  
🚧 SEQ overflow handling  
🚧 Python/Go SDK examples
