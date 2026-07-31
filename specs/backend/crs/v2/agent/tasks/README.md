# Tasks — Agent WebSocket Connections (Phase 2)

Thư mục này chứa **AI-executable tasks** được phân tách từ các [Solutions](../solutions/).

Mỗi task được thiết kế để AI có thể thực thi độc lập với:
- Mục tiêu rõ ràng (1 file tạo hoặc 1 thay đổi cụ thể)
- Code đầy đủ, sẵn sàng copy-paste
- Acceptance criteria kiểm tra được sau khi thực thi

---

## Trạng thái thực thi

> **✅ HOÀN THÀNH: 12/12 tasks DONE (2026-07-26)**  
> **Tests:** 66/66 pass | **TypeScript:** 0 errors  
> **Acceptance Criteria:** 100% ticked (87/87 ACs ✅)

---

## Thứ tự thực thi

```
Phase 1 — Protocol Foundation (SOL-AG-001)
  TASK-001  → src/shared/agent-wire-protocol.ts          [NEW] ✅
  TASK-002  → src/shared/__tests__/agent-wire-protocol.test.ts [NEW] ✅

Phase 2 — Transport Layer (SOL-AG-002)
  TASK-003  → src/main/dev-server/ws-transport.ts        [NEW] ✅
  TASK-004  → src/main/dev-server/ws-handshake.ts        [NEW] ✅
  TASK-005  → src/main/dev-server/__tests__/ws-transport.test.ts [NEW] ✅

Phase 3 — relay-websocket mode (SOL-AG-003)
  TASK-006  → src/main/dev-server/dev-server-relay-bridge.ts [MODIFY: relay-websocket branch] ✅
  TASK-007  → src/main/dev-server/dev-server-manager.ts  [MODIFY: testConnection relay-websocket] ✅

Phase 4 — direct-websocket mode (SOL-AG-004)
  TASK-008  → src/main/dev-server/agent-ws-server.ts     [NEW] ✅
  TASK-009  → src/main/dev-server/__tests__/agent-ws-server.test.ts [NEW] ✅
  TASK-010  → src/main/dev-server/dev-server-relay-bridge.ts [MODIFY: direct-websocket branch] ✅
  TASK-011  → src/main/server-bootstrap.ts               [MODIFY: init AgentWebSocketServer] ✅
  TASK-012  → src/server/index.ts                        [MODIFY: attach AgentWebSocketServer] ✅
```

---

## Dependency Map

```
TASK-001 (agent-wire-protocol)
  └── TASK-002 (test agent-wire-protocol)
  └── TASK-003 (ws-transport)
        └── TASK-004 (ws-handshake)
              └── TASK-005 (test ws-transport + ws-handshake)
              └── TASK-006 (relay-bridge: relay-websocket)
              │     └── TASK-007 (dev-server-manager: testConnection)
              └── TASK-008 (agent-ws-server)
                    └── TASK-009 (test agent-ws-server)
                    └── TASK-010 (relay-bridge: direct-websocket)
                          └── TASK-011 (server-bootstrap)
                                └── TASK-012 (server/index.ts)
```

---

## Tổng số tasks: 12

| Phase | Tasks | Solution | Status | Tests |
|-------|-------|----------|--------|-------|
| Phase 1 — Protocol Foundation | TASK-001 ~ TASK-002 | SOL-AG-001 | ✅ DONE | 15/15 |
| Phase 2 — Transport Layer     | TASK-003 ~ TASK-005 | SOL-AG-002 | ✅ DONE | 21/21 |
| Phase 3 — relay-websocket     | TASK-006 ~ TASK-007 | SOL-AG-003 | ✅ DONE | 19/19 |
| Phase 4 — direct-websocket    | TASK-008 ~ TASK-012 | SOL-AG-004 | ✅ DONE | 11/11 |
