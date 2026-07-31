# TASK-FE-003 — `dev-server-ipc.ts`: Broadcast `agentToken` event lên renderer

**Solution:** [SOL-FE-AG-003](../solutions/SOL-FE-AG-003-ipc-agent-token-event.md)  
**File:** `src/main/ipc/dev-server-ipc.ts` [MODIFY]  
**Depends on:** TASK-FE-002 (manager emits `devServer:agentToken`)  
**Status:** ✅ DONE (2026-07-26)  

---

## Mục tiêu

Thêm vào `registerDevServerIpcHandlers()` để broadcast sự kiện `devServer:agentToken` từ manager lên tất cả renderer windows — giống với cách `devServer:statusChanged` đã làm.

---

## Context hiện tại

```typescript
// src/main/ipc/dev-server-ipc.ts — cuối file, ~line 120+
// ── Push events to renderer ───────────────────────────────────────

manager.on('devServer:statusChanged', (id: string, status: DevServerStatus) => {
  broadcastToAllWindows('devServer:statusChanged', { id, status })
})

manager.on('devServer:added', (id: string) => {
  broadcastToAllWindows('devServer:added', { id })
})

manager.on('devServer:removed', (id: string) => {
  broadcastToAllWindows('devServer:removed', { id })
})
```

---

## Thay đổi cần thực hiện

### File: `src/main/ipc/dev-server-ipc.ts`

**Thêm import:**
```typescript
import type { AgentTokenInfo } from '../../shared/dev-server-types'
```

**Thêm vào cuối `registerDevServerIpcHandlers()`, sau các event broadcasts hiện có:**

```typescript
  // Forward agentTokenGenerated event to renderer
  // Why: when direct-websocket mode is used, DevServerRelayBridge generates
  // a one-time agent token. The renderer needs it immediately so the user
  // can copy the command and start the agent.
  manager.on('devServer:agentToken', (info: AgentTokenInfo) => {
    broadcastToAllWindows('devServer:agentToken', info)
  })
```

**Thêm channel name vào `DEV_SERVER_IPC_CHANNELS`** (nếu cần cleanup):
```typescript
// Line ~11-20: DEV_SERVER_IPC_CHANNELS array
// Không cần thêm vì đây là push event (không dùng ipcMain.handle/removeHandler)
// Push events dùng webContents.send() không cần channel registration
```

---

## Acceptance Criteria

- [x] `manager.on('devServer:agentToken', handler)` được đăng ký trong `registerDevServerIpcHandlers()`
- [x] `broadcastToAllWindows('devServer:agentToken', info)` được gọi khi event fire
- [x] `AgentTokenInfo` được import đúng
- [x] IPC channel name: `'devServer:agentToken'` (dấu `:` — consistent với `devServer:statusChanged`)
- [x] TypeScript compile không lỗi
