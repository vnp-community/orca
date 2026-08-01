# TASK-FE-ORCH-001: Implement IPC Bridge cho Agent Start/Stop/Resume

**Priority:** 🔴 HIGH  
**Effort:** ~90 phút  
**Status:** ✅ DONE — Implemented  
**Bug refs:** BUG-FE-ORCH-001  
**Solution ref:** [SOL-FE-ORCH-001](../solutions/SOL-FE-ORCH-001-ipc-bridge-agent-start-stop-resume.md)

## Mục tiêu

Implement IPC handlers cho `agent:start`, `agent:stop`, `agent:resume` trong Orca main process và wire UI buttons.

## Bước 1 — Tìm IPC handlers hiện tại

```bash
grep -rn "agent.*start\|agent.*stop\|agent.*resume\|ipcMain.handle.*agent" src/main/ --include="*.ts" | head -10
```

## Bước 2 — Thêm IPC handlers (Electron mode)

```typescript
// src/main/ipc/agent-ipc-handlers.ts (NEW)
import { ipcMain } from 'electron'
import type { AgentOrchestrator } from '../agents/AgentOrchestrator'

export function registerAgentIpcHandlers(orchestrator: AgentOrchestrator): void {
  ipcMain.handle('agent:start', async (_, params: { projectId: string; command: string }) => {
    return orchestrator.startAgent(params)
  })

  ipcMain.handle('agent:stop', async (_, params: { agentId: string }) => {
    return orchestrator.stopAgent(params.agentId)
  })

  ipcMain.handle('agent:resume', async (_, params: { agentId: string }) => {
    return orchestrator.resumeAgent(params.agentId)
  })

  ipcMain.handle('agent:list', async () => {
    return orchestrator.listAgents()
  })
}
```

## Bước 3 — Wire UI buttons trong renderer

```typescript
// Trong AgentPanel component:
const startAgent = async () => {
  await window.orcaAPI.invoke('agent:start', { projectId, command })
}
```

## Verification

```bash
pnpm tsc --noEmit
```
