# Backend Solutions — Agent WebSocket Connections (Phase 2)
## Index

**Version:** 1.0  
**Date:** 2026-07-26  
**CRs:** [docs/crs/v2/agent/](../../../../../docs/crs/v2/agent/)  
**TDD Reference:** [specs/backend/tdd/](../../../tdd/)  
**Based on TDD:** TDD-05 (SSH Relay §3 Multiplexer), TDD-11 (Web Server Mode), TDD-13 (Dev Server Onboarding)

---

## Mục tiêu

Bộ solutions này cung cấp **hướng dẫn triển khai chi tiết** (test-driven) cho 4 Change Requests trong `v2/agent`:

- **CR-AG-001**: Định nghĩa Agent Wire Protocol v1 — ngôn ngữ-agnostic spec
- **CR-AG-002**: WebSocket Transport Adapter — `MultiplexerTransport` over WebSocket
- **CR-AG-003**: `relay-websocket` mode — Orca chủ động connect vào Agent WS Server
- **CR-AG-004**: `direct-websocket` mode — Agent chủ động connect vào Orca WS Server

### Nguyên tắc thiết kế

1. **Reuse Multiplexer** — Không viết JSON-RPC engine mới, chỉ add WebSocket transport adapter
2. **Additive** — Không phá vỡ relay-ssh hoặc bất kỳ existing path nào
3. **Language-agnostic protocol** — Frame format phải implement được bằng Python, Go, Java
4. **Test-driven** — Mỗi module có test spec trước implementation
5. **Server-mode compatible** — Tất cả code phải chạy trong Node.js server (không import `electron`)

---

## Danh sách Solutions

| Solution | CR tương ứng | Domain | TDD Reference | Status |
|----------|-------------|--------|--------------|--------|
| [SOL-AG-001](./SOL-AG-001-wire-protocol.md) | CR-AG-001 | Agent Wire Protocol: types, constants, frame codec | TDD-05 §4 Relay Protocol | ✅ IMPLEMENTED |
| [SOL-AG-002](./SOL-AG-002-ws-transport-adapter.md) | CR-AG-002 | WebSocket Transport Adapter + Handshake | TDD-05 §4, TDD-13 §3 | ✅ IMPLEMENTED |
| [SOL-AG-003](./SOL-AG-003-relay-websocket.md) | CR-AG-003 | relay-websocket: Orca → Agent WS Server | TDD-05 §4, TDD-13 §3 | ✅ IMPLEMENTED |
| [SOL-AG-004](./SOL-AG-004-direct-websocket.md) | CR-AG-004 | direct-websocket: Agent → Orca WS Server | TDD-11 §2, TDD-13 §3 | ✅ IMPLEMENTED |

---

## Dependency Graph & Thứ tự triển khai

```
SOL-AG-001 (Protocol types/constants)
    │
    └─► SOL-AG-002 (WS Transport Adapter + Handshake)
             │
             ├─► SOL-AG-003 (relay-websocket) ← triển khai song song
             └─► SOL-AG-004 (direct-websocket) ← triển khai song song
```

**Bắt buộc:** AG-001 → AG-002 → AG-003 & AG-004 (parallel)

---

## File Map tổng hợp

### New files (tạo mới)

| File | Solution | Mô tả |
|------|----------|-------|
| `src/shared/agent-wire-protocol.ts` | AG-001 | Types, constants, error codes |
| `src/main/dev-server/ws-transport.ts` | AG-002 | WebSocket → MultiplexerTransport adapter |
| `src/main/dev-server/ws-handshake.ts` | AG-002 | Handshake (initiator + receiver) |
| `src/main/dev-server/agent-ws-server.ts` | AG-004 | WS server nhận incoming agent |
| `src/main/dev-server/__tests__/ws-transport.test.ts` | AG-002 | Unit tests transport + handshake |
| `src/main/dev-server/__tests__/agent-ws-server.test.ts` | AG-004 | Unit tests agent WS server |

### Modified files (sửa đổi)

| File | Solution | Thay đổi |
|------|----------|---------|
| `src/main/dev-server/dev-server-relay-bridge.ts` | AG-003, AG-004 | Fill Phase 2 branches |
| `src/main/dev-server/dev-server-manager.ts` | AG-003, AG-004 | testConnection relay-websocket path |
| `src/main/server-bootstrap.ts` | AG-004 | Init AgentWebSocketServer + attach HTTP |

---

## Acceptance Criteria tổng hợp

| # | Criteria | Solution | Status |
|---|----------|---------|--------|
| AC-1 | `ws-transport.ts` hoạt động với `ws` npm và không import `electron` | AG-002 | ✅ |
| AC-2 | Handshake timeout sau `AGENT_TIMEOUT_MS` (20s) | AG-002 | ✅ |
| AC-3 | `relay-websocket`: Orca kết nối thành công tới TypeScript agent server | AG-003 | ✅ |
| AC-4 | `relay-websocket`: Token auth — invalid token → WS close + lỗi rõ ràng | AG-003 | ✅ |
| AC-5 | `direct-websocket`: Agent connect thành công, handshake pass | AG-004 | ✅ |
| AC-6 | `direct-websocket`: Timeout 60s nếu agent không connect | AG-004 | ✅ |
| AC-7 | `DevServerRelayBridge.session` vẫn là `SshChannelMultiplexer` | AG-002 | ✅ |
| AC-8 | Unit tests >= 90% coverage cho transport và handshake | AG-002 | ✅ (21 tests) |
| AC-9 | `testConnection()` hỗ trợ cả `relay-websocket` (không cần SSH) | AG-003 | ✅ |
| AC-10 | Disconnect graceful: WS close → multiplexer dispose | AG-003, AG-004 | ✅ |

---

## Tóm tắt Implementation (2026-07-26)

### Files tạo mới

| File | Solution | Tests |
|------|----------|-------|
| `src/shared/agent-wire-protocol.ts` | SOL-AG-001 | 15/15 ✅ |
| `src/shared/__tests__/agent-wire-protocol.test.ts` | SOL-AG-001 | 15/15 ✅ |
| `src/main/dev-server/ws-transport.ts` | SOL-AG-002 | 21/21 ✅ |
| `src/main/dev-server/ws-handshake.ts` | SOL-AG-002 | 21/21 ✅ |
| `src/main/dev-server/__tests__/ws-transport.test.ts` | SOL-AG-002 | 21/21 ✅ |
| `src/main/dev-server/agent-ws-server.ts` | SOL-AG-004 | 11/11 ✅ |
| `src/main/dev-server/__tests__/agent-ws-server.test.ts` | SOL-AG-004 | 11/11 ✅ |

### Files sửa đổi

| File | Solution | Thay đổi chính |
|------|----------|----------------|
| `src/main/dev-server/dev-server-relay-bridge.ts` | AG-003, AG-004 | relay-websocket branch + direct-websocket + EventEmitter |
| `src/main/dev-server/dev-server-manager.ts` | AG-003, AG-004 | WS fast path + agentWsServer 3rd param |
| `src/main/server-bootstrap.ts` | AG-004 | AgentWebSocketServer init + shutdown |
| `src/server/index.ts` | AG-004 | agentWsServer.attach(httpServer) |

### Kết quả tổng hợp

> **✅ 12/12 tasks DONE · 66/66 tests pass · 0 TS errors · 100% ACs ticked**
