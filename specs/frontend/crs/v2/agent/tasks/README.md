# Tasks — Frontend Agent WebSocket Connections (Phase 2)

Thư mục này chứa **AI-executable tasks** được phân tách từ [Solutions](../solutions/).

Mỗi task được thiết kế để AI có thể thực thi độc lập với:
- Mục tiêu rõ ràng (1 file tạo hoặc 1 thay đổi cụ thể)
- Code đầy đủ, sẵn sàng copy-paste
- Acceptance criteria kiểm tra được sau khi thực thi

---

## Trạng thái thực thi

> **✅ 9/9 tasks DONE (ALL PHASES COMPLETE)**  
> **Tests:** 66/66 pass | **TypeScript:** 0 errors

---

## Thứ tự thực thi

```
Phase A — Shared types & IPC bridge (no UI deps)
  TASK-FE-001  → src/shared/dev-server-types.ts          [MODIFY: AgentTokenInfo type]
  TASK-FE-002  → src/main/dev-server/dev-server-manager.ts [MODIFY: re-emit agentTokenGenerated]
  TASK-FE-003  → src/main/ipc/dev-server-ipc.ts          [MODIFY: broadcast agentToken event]
  TASK-FE-004  → src/preload/preload.ts                  [MODIFY: onAgentToken / offAgentToken]
  TASK-FE-005  → src/renderer/src/web/web-preload-api.ts [MODIFY: onAgentToken web mode]

Phase B — Hooks (depends on Phase A)
  TASK-FE-006  → src/renderer/src/hooks/useAddDevServer.ts [MODIFY: agentToken state]

Phase C — UI Components (depends on Phase B)
  TASK-FE-007  → src/renderer/src/components/dev-server/AgentTokenPanel.tsx [NEW]
  TASK-FE-008  → src/renderer/src/components/dev-server/AddDevServerDialog.tsx [MODIFY]
  TASK-FE-009  → src/renderer/src/components/dev-server/DevServerCard.tsx [MODIFY: mode badge]
```

---

## Dependency Map

```
TASK-FE-001 (AgentTokenInfo type)
  └── TASK-FE-002 (manager re-emit)
        └── TASK-FE-003 (IPC broadcast)
              └── TASK-FE-004 (preload channels)
                    └── TASK-FE-005 (web-preload-api)
                          └── TASK-FE-006 (useAddDevServer agentToken state)
                                └── TASK-FE-007 (AgentTokenPanel component)
                                      └── TASK-FE-008 (AddDevServerDialog)
TASK-FE-009 (DevServerCard badge) ← standalone, không phụ thuộc Phase A/B
```

---

## Tổng số tasks: 9

| Phase | Tasks | Solution | Status |
|-------|-------|----------|--------|
| Phase A — Types & IPC bridge | TASK-FE-001 ~ TASK-FE-005 | SOL-FE-AG-003 | ✅ DONE |
| Phase B — Hooks | TASK-FE-006 | SOL-FE-AG-002 | ✅ DONE |
| Phase C — UI | TASK-FE-007 ~ TASK-FE-009 | SOL-FE-AG-001,002,004 | ✅ DONE |
