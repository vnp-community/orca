# CR-AG-000 — Gap Analysis: Phase 2 WebSocket Agent Connections

**CR-ID:** CR-AG-000  
**Ngày:** 2026-07-26  
**Priority:** 🔴 Critical  
**Effort:** Analysis (đọc trước CR-AG-001 → CR-AG-004)  
**Status:** ✅ Implemented (2026-07-26)  

**Implementation Note:** Gap analysis đã được implement đầy đủ theo roadmap: CR-AG-001→002→003→004  
**Backend:** SOL-AG-001 ~ SOL-AG-004 (IMPLEMENTED)  
**Frontend:** SOL-FE-AG-001 ~ SOL-FE-AG-004 (IMPLEMENTED)  

---

## 1. Bối cảnh

Phase 1 chỉ hỗ trợ `relay-ssh`: Orca deploy relay lên dev server qua SSH rồi giao tiếp qua SSH exec channel stdin/stdout.

Phase 2 mở ra 2 mode mới:

| Mode | Ai connect đến ai | Use case |
|------|-------------------|----------|
| `relay-websocket` | **Orca** → `ws://agent:port` | Agent tự khởi WS server, Orca pull vào |
| `direct-websocket` | **Agent** → `ws://orca:6768` | Agent chủ động kết nối vào Orca |

Yêu cầu đặc biệt: **Protocol phải đủ chuẩn để agent có thể viết bằng TypeScript, Python, Go, Java** mà không cần biết internals của Orca.

---

## 2. Các vấn đề hiện tại (GAP)

### GAP-1: `DevServerRelayBridge.connect()` throw hardcoded error

```typescript
// src/main/dev-server/dev-server-relay-bridge.ts:85-89
throw new Error(
  `Connection type '${this.config.connectionType}' is not yet implemented. ` +
  `Only 'relay-ssh' is supported in Phase 1.`
)
```

### GAP-2: `SshChannelMultiplexer` được hardcode cho SSH transport

`MultiplexerTransport` interface đang đủ trừu tượng (write/onData/onClose), nhưng **không có WebSocket adapter** implement nó.

### GAP-3: Không có WS server endpoint cho agent kết nối vào Orca

`src/server/index.ts` cấu hình WS port `6768` nhưng đây là RPC port cho **browser client**, không phải cho agent. Chưa có auth mechanism cho agent.

### GAP-4: Không có protocol spec chuẩn cho agent bên ngoài

`relay-protocol.ts` là TypeScript nội bộ, không có spec ngôn ngữ-agnostic (không có OpenAPI/AsyncAPI/protobuf/markdown spec).

### GAP-5: Không có auth/token mechanism cho WebSocket connections

SSH dùng key-pair auth. WebSocket cần cơ chế token/secret khác.

---

## 3. Danh sách Change Requests

| CR | Tên | Loại | Effort |
|----|-----|------|--------|
| [CR-AG-001](./CR-AG-001-wire-protocol-spec.md) | Agent Wire Protocol Specification | Protocol Spec | M |
| [CR-AG-002](./CR-AG-002-ws-transport-adapter.md) | WebSocket Transport Adapter cho Multiplexer | Backend | M |
| [CR-AG-003](./CR-AG-003-relay-websocket-mode.md) | relay-websocket: Orca → Agent WS Server | Backend | L |
| [CR-AG-004](./CR-AG-004-direct-websocket-mode.md) | direct-websocket: Agent → Orca WS Server | Backend | L |

---

## 4. Dependency graph

```
CR-AG-001 (Protocol Spec)
    └─► CR-AG-002 (WS Transport Adapter)
             ├─► CR-AG-003 (relay-websocket mode)
             └─► CR-AG-004 (direct-websocket mode)
```

**Thứ tự triển khai bắt buộc**: AG-001 → AG-002 → AG-003 & AG-004 (parallel).
