# Phase 2: Agent WebSocket Connections — Change Requests

> **Phạm vi:** Implement `relay-websocket` và `direct-websocket` trong `DevServerRelayBridge`,  
> thiết lập giao thức chuẩn để agent có thể viết bằng **TypeScript, Python, Go, Java**.

---


---

## ✅ Implementation Status (2026-07-26)

**Tất cả 5 CRs đã được implement đầy đủ.**

| Layer | Phạm vi | Tests | Status |
|-------|---------|-------|--------|
| Backend | Wire Protocol, WsTransport, WsHandshake, relay-websocket, direct-websocket, AgentWebSocketServer, IPC bridge | 66/66 pass | ✅ DONE |
| Frontend | AddDevServerDialog (3 modes), AgentTokenPanel, useAddDevServer, DevServerCard badge | 0 TS errors | ✅ DONE |

## Tổng quan kiến trúc Phase 2

```
Mode 1: relay-websocket (Orca → Agent)
──────────────────────────────────────
  Agent WS Server                     Orca App
  ws://agent:6799/orca-relay          │
         │ ◄─── Orca connect ─────────┘
         │ ◄─── Bearer token ─────────
         │ agent.handshake ──────────►
         │ ◄── handshake-ok ──────────
         │◄══ JSON-RPC (framed) ═════►│

Mode 2: direct-websocket (Agent → Orca)
────────────────────────────────────────
  Agent                               Orca WS Server
         │                            ws://orca:6768/agent
         ├─── WebSocket connect ──────►
         ├─── agent.handshake ────────►  (agentToken validated)
         │◄── handshake-ok ────────────
         │◄══ JSON-RPC (framed) ══════►│
```

---

## Danh sách Change Requests

| CR | Mô tả | Effort | Depends on | Status |
|----|-------|--------|------------|--------|
| [CR-AG-000](./CR-AG-000-gap-analysis.md) | Gap Analysis — vấn đề hiện tại và roadmap | — | — | ✅ Implemented |
| [CR-AG-001](./CR-AG-001-wire-protocol-spec.md) | Agent Wire Protocol Spec (ngôn ngữ-agnostic) | M | — | ✅ Implemented |
| [CR-AG-002](./CR-AG-002-ws-transport-adapter.md) | WebSocket Transport Adapter cho Multiplexer | M | AG-001 | ✅ Implemented |
| [CR-AG-003](./CR-AG-003-relay-websocket-mode.md) | relay-websocket: Orca → Agent WS Server | L | AG-002 | ✅ Implemented |
| [CR-AG-004](./CR-AG-004-direct-websocket-mode.md) | direct-websocket: Agent → Orca WS Server | L | AG-002 | ✅ Implemented |

---

## Dependency graph

```
CR-AG-001 (Wire Protocol Spec)
    │
    └─► CR-AG-002 (WS Transport Adapter)
             │
             ├─► CR-AG-003 (relay-websocket)   ← parallel
             └─► CR-AG-004 (direct-websocket)  ← parallel
```

**Thứ tự triển khai:** AG-001 → AG-002 → AG-003 & AG-004 (đồng thời).

---

## Key design decisions

### 1. Reuse `SshChannelMultiplexer`
Không tạo mux mới — adapter pattern: `createWebSocketTransport()` wrap WebSocket vào `MultiplexerTransport` interface hiện có. Toàn bộ JSON-RPC framing, keepalive, timeout được kế thừa.

### 2. Binary WebSocket frames (không dùng text)
Cùng 13-byte header format như SSH transport. Agent phải dùng `ws.binaryType = 'nodebuffer'` / nhận binary messages.

### 3. Handshake trước khi mux
`agent.handshake` được thực hiện **before** `SshChannelMultiplexer` được wired — tránh race condition giữa handshake và regular RPC calls.

### 4. Auth mechanism
- `relay-websocket`: Bearer token trong HTTP `Authorization` header (WS upgrade request)  
- `direct-websocket`: `agentToken` field trong `agent.handshake` params

### 5. Language-agnostic frame encoding
13-byte header: `[TYPE(1)] [SEQ u32 BE] [ACK u32 BE] [LEN u32 BE]`  
Payload: UTF-8 JSON. Trivial implement bằng bất kỳ ngôn ngữ nào có `struct.pack` / `ByteBuffer`.

---

## Files sẽ được tạo/sửa

### New files
| File | CR | Mô tả |
|------|----|-------|
| `src/shared/agent-wire-protocol.ts` | AG-001 | Constants và types cho agent protocol |
| `src/main/dev-server/ws-transport.ts` | AG-002 | WebSocket → MultiplexerTransport adapter |
| `src/main/dev-server/ws-handshake.ts` | AG-002 | Handshake logic (initiator + receiver) |
| `src/main/dev-server/agent-ws-server.ts` | AG-004 | WS server nhận incoming agent connections |
| `docs/specs/agent-wire-protocol-v1.md` | AG-001 | Public spec cho external agent developers |
| `docs/specs/agent-relay-websocket-server.md` | AG-003 | Guide: viết agent WS server |
| `docs/specs/agent-direct-websocket-client.md` | AG-004 | Guide: viết agent WS client |

### Modified files
| File | CR | Thay đổi |
|------|----|---------|
| `src/main/dev-server/dev-server-relay-bridge.ts` | AG-003, AG-004 | Fill Phase 2 branches |
| `src/main/dev-server/dev-server-manager.ts` | AG-004 | Inject AgentWebSocketServer |
| `src/main/server-bootstrap.ts` | AG-004 | Khởi tạo AgentWebSocketServer |
| `src/renderer/.../DevServerPane.tsx` | AG-003, AG-004 | UI cho WS config + token display |
