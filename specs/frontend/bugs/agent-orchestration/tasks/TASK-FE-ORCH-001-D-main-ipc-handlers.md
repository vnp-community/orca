# TASK-FE-ORCH-001-D: Main IPC Handlers — `agent:start`, `agent:stop`, `agent:resume`

**Domain:** agent-orchestration  
**Solution Ref:** SOL-FE-ORCH-001 Bước 4  
**Priority:** 🔴 P0  
**Estimated:** 30 phút  
**Status:** ✅ DONE — Implemented in main/ipc/agent-orchestration.ts

---

## Mục tiêu

Tạo file IPC handler cho Electron Main process để xử lý `agent:start`, `agent:stop`, `agent:resume`.

---

## Files cần tạo/sửa

1. **TẠO MỚI:** `src/main/ipc/agent-orchestration-ipc.ts`
2. **MODIFY:** `src/main/index.ts` — đăng ký handlers

---

## Bước 1: Tạo `src/main/ipc/agent-orchestration-ipc.ts`

```typescript
import { ipcMain } from 'electron'
import type { AgentStartOptions, AgentStopOptions, AgentResumeOptions } from '../../shared/types/api-types'

export function registerAgentOrchestrationIpc(agentManager: AgentManager): void {
  ipcMain.handle('agent:start', async (_event, opts: AgentStartOptions) => {
    return agentManager.start(opts)
  })

  ipcMain.handle('agent:stop', async (_event, opts: AgentStopOptions) => {
    return agentManager.stop(opts)
  })

  ipcMain.handle('agent:resume', async (_event, opts: AgentResumeOptions) => {
    return agentManager.resume(opts)
  })
}
```

> **Lưu ý:** Tìm `AgentManager` hoặc class tương đương đang tồn tại trong codebase. Dùng `grep -r "class AgentManager\|agentManager" src/main/` để locate.

## Bước 2: Đăng ký trong `src/main/index.ts`

Thêm import và call trong `app.whenReady()`:

```typescript
import { registerAgentOrchestrationIpc } from './ipc/agent-orchestration-ipc'
// ...
app.whenReady().then(() => {
  // ... existing setup ...
  registerAgentOrchestrationIpc(agentManager)
})
```

---

## Verify

```bash
grep -r "agent:start" src/main/
```

## Depends on
Không có (độc lập với Renderer tasks)

## Blocking
TASK-FE-ORCH-001-E (end-to-end test)
