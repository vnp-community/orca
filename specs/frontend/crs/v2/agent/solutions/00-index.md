# Frontend Solutions — Agent WebSocket Connections (Phase 2)
## Index

**Version:** 1.0  
**Date:** 2026-07-26  
**CRs:** [docs/crs/v2/agent/](../../../../../docs/crs/v2/agent/)  
**TDD Reference:** [specs/frontend/tdd/](../../../tdd/)  
**Backend Solutions:** [specs/backend/crs/v2/agent/solutions/](../../../../backend/crs/v2/agent/solutions/)  
**Based on TDD:** TDD-FE-09 (Onboarding/DevServer), TDD-FE-07 (Hooks/IPC)

---

## Mục tiêu

Frontend cần cập nhật UI để hỗ trợ hai connection mode mới (`relay-websocket`, `direct-websocket`) mà backend đã implement. Cụ thể:

1. **SOL-FE-AG-001** — `relay-websocket`: Sửa `AddDevServerDialog` để hiển thị WebSocket URL input và hướng dẫn cụ thể
2. **SOL-FE-AG-002** — `direct-websocket`: Hiển thị `agentToken` + lệnh khởi động agent sau khi Connect
3. **SOL-FE-AG-003** — IPC Bridge: Expose `agentTokenGenerated` event từ backend lên renderer và listen trong `useIpcEvents`
4. **SOL-FE-AG-004** — `DevServerCard`: Hiển thị connection type badge + agent token panel khi `direct-websocket`

### Nguyên tắc thiết kế

1. **Backend-first** — UI phản ánh backend state, không tự generate token
2. **Additive** — Không phá vỡ UI relay-ssh hiện tại
3. **EventEmitter → IPC → Renderer** — Luồng sự kiện: `DevServerRelayBridge.emit('agentTokenGenerated')` → IPC → `window.api.devServer.onAgentToken()`
4. **Platform guard** — Web mode và Electron mode đều được support
5. **No electron import** — Tất cả code renderer không dùng `electron` API trực tiếp

---

## Danh sách Solutions

| Solution | CR tương ứng | Domain | TDD Reference | Status |
|----------|-------------|--------|--------------|--------|
| [SOL-FE-AG-001](./SOL-FE-AG-001-relay-websocket-ui.md) | CR-AG-003 | AddDevServerDialog: relay-websocket UX | TDD-FE-09 §7 | ✅ IMPLEMENTED |
| [SOL-FE-AG-002](./SOL-FE-AG-002-direct-websocket-token-ui.md) | CR-AG-004 | Direct-websocket token display + agent command | TDD-FE-09 §7 | ✅ IMPLEMENTED |
| [SOL-FE-AG-003](./SOL-FE-AG-003-ipc-agent-token-event.md) | CR-AG-004 | IPC bridge: agentTokenGenerated event | TDD-FE-07 §2 | ✅ IMPLEMENTED |
| [SOL-FE-AG-004](./SOL-FE-AG-004-devserver-card-mode-badge.md) | CR-AG-003, CR-AG-004 | DevServerCard: mode badge + reconnect info | TDD-FE-09 §7 | ✅ IMPLEMENTED |

---

## Dependency Graph & Thứ tự triển khai

```
Backend: SOL-AG-004 (AgentWebSocketServer, emits agentTokenGenerated)
    │
    ▼
SOL-FE-AG-003  (IPC bridge: devServer.onAgentToken → renderer)
    │
    ├─► SOL-FE-AG-002  (UI hiển thị token + command khi direct-websocket)
    │
SOL-FE-AG-001  (UI: relay-websocket hint text — standalone, no deps)
SOL-FE-AG-004  (DevServerCard: mode badge — standalone, no deps)
```

---

## File Map tổng hợp

### Modified files (sửa đổi)

| File | Solution | Thay đổi |
|------|----------|---------|
| `src/renderer/src/components/dev-server/AddDevServerDialog.tsx` | FE-AG-001, FE-AG-002 | relay-ws hint + direct-ws token panel |
| `src/renderer/src/components/dev-server/DevServerCard.tsx` | FE-AG-004 | Connection type badge |
| `src/renderer/src/hooks/useAddDevServer.ts` | FE-AG-002 | agentToken state, onAgentToken subscription |
| `src/renderer/src/hooks/useIpcEvents.ts` | FE-AG-003 | Subscribe devServer.onAgentToken |
| `src/renderer/src/web/web-preload-api.ts` | FE-AG-003 | Expose onAgentToken/offAgentToken via RPC poll |
| `src/preload/preload.ts` | FE-AG-003 | Expose onAgentToken IPC channel |
| `src/main/ipc/dev-server-ipc.ts` | FE-AG-003 | Forward agentTokenGenerated → IPC |

### New files (tạo mới)

| File | Solution | Mô tả |
|------|----------|-------|
| `src/renderer/src/components/dev-server/AgentTokenPanel.tsx` | FE-AG-002 | Panel hiển thị token + lệnh copy |

---

## Acceptance Criteria tổng hợp

| # | Criteria | Solution |
|---|----------|---------|
| AC-FE-1 | `relay-websocket`: Dialog hiển thị URL input với placeholder `ws://devserver:6799/orca-relay?token=...` | FE-AG-001 |
| AC-FE-2 | `relay-websocket`: Có hint text giải thích agent cần chạy WS server | FE-AG-001 |
| AC-FE-3 | `direct-websocket`: Sau Connect, hiển thị `agentToken` dạng copyable command | FE-AG-002 |
| AC-FE-4 | `direct-websocket`: Timeout 60s — nếu agent không connect, hiển thị lỗi rõ ràng | FE-AG-002 |
| AC-FE-5 | `agentTokenGenerated` event từ backend được forward lên renderer qua IPC | FE-AG-003 |
| AC-FE-6 | `DevServerCard` hiển thị mode badge (`SSH`, `WS→`, `←WS`) | FE-AG-004 |
| AC-FE-7 | `useAddDevServer.agentToken` state được reset khi dialog đóng | FE-AG-002 |
| AC-FE-8 | `direct-websocket` canTest = `true` (không cần wsUrl — chờ agent connect) | FE-AG-001 |
