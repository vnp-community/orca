# TASK-TM-003-C: IPC handlers — `terminal.session.save/restore/delete`

**Domain:** terminal-management  
**Solution Ref:** SOL-TM-003 Phần 3  
**Bug:** BUG-TM-003  
**Priority:** 🟠 P1  
**Estimated:** 25 phút  
**Status:** ✅ DONE — Implemented

---

## Mục tiêu

Expose `terminal.session.save`, `terminal.session.restore`, `terminal.session.delete` qua IPC để Renderer gọi được.

---

## Files cần tạo/sửa

Tìm pattern IPC registration hiện có:
```bash
ls src/main/ipc/
```

- **TẠO MỚI hoặc MODIFY:** `src/main/ipc/terminal-session-ipc.ts`
- **MODIFY:** `src/main/index.ts`

---

## Các bước thực thi

```typescript
// src/main/ipc/terminal-session-ipc.ts
export function registerTerminalSessionIpc(service: TerminalSessionService) {
  ipcMain.handle('terminal.session.save', async (_evt, params) => {
    await service.saveSession(params)
    return { ok: true }
  })

  ipcMain.handle('terminal.session.restore', async (_evt, { worktreeId, terminalId }) => {
    return service.restoreSession(worktreeId, terminalId)
  })

  ipcMain.handle('terminal.session.delete', async (_evt, { worktreeId, terminalId }) => {
    await service.deleteSession(worktreeId, terminalId)
    return { ok: true }
  })
}
```

Thêm vào `web-preload-api.ts` / preload bridge nếu cần Web mode.

---

## Verify

```bash
grep -n "terminal.session" src/main/ipc/terminal-session-ipc.ts
```

## Depends on
TASK-TM-003-B (service)

## Blocking
TASK-TM-003-D (Renderer integration)
