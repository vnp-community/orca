# SOL-FE-ORCH-001: Thêm IPC Bridge cho `agent.start`, `agent.stop`, `agent.resume`

## Bug Reference
- **Bug:** BUG-FE-ORCH-001
- **Mức độ:** 🔴 HIGH
- **TDD Reference:** TDD-FE-07 (Custom Hooks & IPC Events), TDD-FE-01 (Architecture Overview §2.3 `window.api`)

---

## Root Cause

Renderer không có IPC bridge cho các action `agent.start` / `agent.stop` / `agent.resume`. Flow HLD (BL-AG-01, BL-AG-02, BL-AG-03) yêu cầu:
```
[Renderer] click "Start Agent"
    contextBridge.invoke('agent.start', { worktreeId, agentType, trustPreset })
```
Nhưng không có handler nào trong `src/preload/` lẫn `src/main/ipc/`.

---

## Giải pháp

### Bước 1 — Mở rộng `OrcaApi` Interface

**File:** `src/renderer/src/web/web-preload-api.ts` (Web mode)  
**File:** `src/preload/index.ts` (Electron mode)

Thêm namespace `agent` vào interface `OrcaApi`:

```typescript
// Thêm vào OrcaApi interface (src/renderer/src/web/web-preload-api.ts)
interface OrcaApi {
  // ... existing ...
  agent: {
    start: (opts: AgentStartOptions) => Promise<AgentStartResult>
    stop:  (opts: AgentStopOptions)  => Promise<void>
    resume: (opts: AgentResumeOptions) => Promise<AgentResumeResult>
  }
}

// Types (src/renderer/src/web/web-preload-api.ts hoặc shared types file):
interface AgentStartOptions {
  worktreeId: string
  agentType: 'claude' | 'codex' | 'custom'
  trustPreset: 'standard' | 'permissive' | 'strict'
}

interface AgentStartResult {
  sessionId: string
  status: 'started' | 'already-running'
}

interface AgentStopOptions {
  sessionId: string
}

interface AgentResumeOptions {
  sessionId: string
}

interface AgentResumeResult {
  resumed: boolean
}
```

---

### Bước 2 — Preload Bridge (Electron Desktop)

**File:** `src/preload/index.ts`

```typescript
// Thêm vào contextBridge.exposeInMainWorld('api', { ... })
agent: {
  start:  (opts: AgentStartOptions)  => ipcRenderer.invoke('agent:start',  opts),
  stop:   (opts: AgentStopOptions)   => ipcRenderer.invoke('agent:stop',   opts),
  resume: (opts: AgentResumeOptions) => ipcRenderer.invoke('agent:resume', opts),
},
```

---

### Bước 3 — Web Preload API (Web/Browser mode)

**File:** `src/renderer/src/web/web-preload-api.ts`

```typescript
// Trong installWebPreloadApi() hoặc buildWebApi()
agent: {
  start: (opts) => rpcClient.call('agent.start', opts),
  stop:  (opts) => rpcClient.call('agent.stop',  opts),
  resume: (opts) => rpcClient.call('agent.resume', opts),
},
```

---

### Bước 4 — Main IPC Handlers (Electron)

**File:** `src/main/ipc/agent-orchestration-ipc.ts` (TẠO MỚI)

```typescript
import { ipcMain } from 'electron'
import { AgentManager } from '../agents/AgentManager'

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

**File:** `src/main/index.ts` — đăng ký handler khi app ready:

```typescript
import { registerAgentOrchestrationIpc } from './ipc/agent-orchestration-ipc'

app.whenReady().then(() => {
  // ... existing setup ...
  registerAgentOrchestrationIpc(agentManager)
})
```

---

### Bước 5 — Worktree Card UI Integration

**File:** `src/renderer/src/components/worktree/WorktreeCard.tsx` (hoặc tương đương)

```typescript
// Thay thế terminal-based launch hiện tại bằng IPC call:
const handleStartAgent = async () => {
  try {
    const result = await window.api.agent.start({
      worktreeId: worktree.id,
      agentType: selectedAgentType,
      trustPreset: 'standard',
    })
    // Update store với sessionId
    useAppStore.getState().updateAgentStatus({
      worktreeId: worktree.id,
      sessionId: result.sessionId,
      status: 'running',
    })
    toast.success(`Agent started (session: ${result.sessionId})`)
  } catch (err) {
    toast.error(`Failed to start agent: ${(err as Error).message}`)
  }
}

const handleStopAgent = async () => {
  if (!currentSession?.sessionId) return
  try {
    await window.api.agent.stop({ sessionId: currentSession.sessionId })
    useAppStore.getState().updateAgentStatus({
      worktreeId: worktree.id,
      status: 'stopped',
    })
    toast.success('Agent stopped')
  } catch (err) {
    toast.error(`Failed to stop agent: ${(err as Error).message}`)
  }
}
```

---

### Bước 6 — useIpcEvents: Subscribe Agent Events từ Backend

**File:** `src/renderer/src/hooks/useIpcEvents.ts`

Theo pattern TDD-FE-07 §2, thêm subscription cho agent events:

```typescript
// Trong useEffect callback của useIpcEvents():
window.api.agent?.onStatusChanged?.((event) => {
  store.updateAgentStatus(event)
})

// Preload cần expose:
agent: {
  // ... start/stop/resume ...
  onStatusChanged: (cb: (event: AgentStatusEvent) => void) => 
    ipcRenderer.on('agent:statusChanged', (_, e) => cb(e)),
  offStatusChanged: (cb) => ipcRenderer.removeListener('agent:statusChanged', cb),
}
```

---

---

### Bước 7 — AgentStatus Zustand Slice (BL-AG-01)

**File:** `src/renderer/src/store/slices/agent-status.ts` (MODIFY — thêm remote agent session tracking)

Theo TDD-FE-02 §2, đã có `agent-status` slice (~105K). Cần **mở rộng** để track remote sessions:

```typescript
// Thêm vào AgentStatusSlice:
type RemoteAgentSession = {
  sessionId: string
  worktreeId: string
  agentType: 'claude' | 'codex' | 'custom'
  status: 'starting' | 'running' | 'stopped' | 'error'
  startedAt: number
  stoppedAt?: number
  errorMessage?: string
}

// Extensions to existing AgentStatusSlice:
type AgentStatusSliceExtension = {
  // Remote agent sessions (IPC-based, BL-AG-01/02/03)
  remoteAgentSessions: Record<string, RemoteAgentSession>  // key: worktreeId
  setRemoteAgentSession: (worktreeId: string, session: RemoteAgentSession) => void
  clearRemoteAgentSession: (worktreeId: string) => void
  updateAgentStatus: (event: AgentStatusEvent) => void  // ← called by useIpcEvents
}

type AgentStatusEvent = {
  worktreeId: string
  sessionId?: string
  status: RemoteAgentSession['status']
  errorMessage?: string
}

// Slice implementation (thêm vào createAgentStatusSlice):
createAgentStatusSlice: StateCreator<AppState, [], [], AgentStatusSliceExtension> = (set) => ({
  remoteAgentSessions: {},
  setRemoteAgentSession: (worktreeId, session) =>
    set(s => ({ remoteAgentSessions: { ...s.remoteAgentSessions, [worktreeId]: session } })),
  clearRemoteAgentSession: (worktreeId) =>
    set(s => {
      const { [worktreeId]: _, ...rest } = s.remoteAgentSessions
      return { remoteAgentSessions: rest }
    }),
  updateAgentStatus: (event) =>
    set(s => {
      const existing = s.remoteAgentSessions[event.worktreeId]
      if (!existing && !event.sessionId) return s
      const updated: RemoteAgentSession = {
        ...(existing ?? { worktreeId: event.worktreeId, agentType: 'claude', startedAt: Date.now() }),
        sessionId: event.sessionId ?? existing?.sessionId ?? '',
        status: event.status,
        ...(event.errorMessage ? { errorMessage: event.errorMessage } : {}),
        ...(event.status === 'stopped' ? { stoppedAt: Date.now() } : {}),
      }
      return { remoteAgentSessions: { ...s.remoteAgentSessions, [event.worktreeId]: updated } }
    }),
})
```

---

### Bước 8 — AgentPanel UI Component

**File:** `src/renderer/src/components/workspace/AgentPanel.tsx` (TẠO MỚI)

Theo TDD-FE-01 §v5.0 Module Structure, cần `AgentPanel.tsx` trong `components/workspace/`:

```typescript
// src/renderer/src/components/workspace/AgentPanel.tsx
// Agent start/stop/resume UI integrated với IPC bridge (BUG-FE-ORCH-001 fix)

import { useState } from 'react'
import { useAppStore } from '@/store'
import { useShallow } from 'zustand/react/shallow'
import { useWorkspace } from '@/context/WorkspaceContext'
import { Button } from '@/components/ui/button'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Badge } from '@/components/ui/badge'
import { Play, Square, RotateCcw, Loader2 } from 'lucide-react'
import { toast } from 'sonner'

type AgentType = 'claude' | 'codex' | 'custom'

export function AgentPanel() {
  const { currentWorktree } = useWorkspace()
  const worktreeId = currentWorktree?.id ?? ''

  const { session, updateAgentStatus, setRemoteAgentSession } = useAppStore(
    useShallow(s => ({
      session: s.remoteAgentSessions[worktreeId],
      updateAgentStatus: s.updateAgentStatus,
      setRemoteAgentSession: s.setRemoteAgentSession,
    }))
  )

  const [agentType, setAgentType] = useState<AgentType>('claude')
  const [isActing, setIsActing] = useState(false)

  const startAgent = async () => {
    if (!worktreeId) return
    setIsActing(true)
    // Optimistic: show 'starting' immediately
    setRemoteAgentSession(worktreeId, {
      sessionId: '',
      worktreeId,
      agentType,
      status: 'starting',
      startedAt: Date.now(),
    })
    try {
      const result = await window.api.agent.start({
        worktreeId,
        agentType,
        trustPreset: 'standard',
      })
      updateAgentStatus({ worktreeId, sessionId: result.sessionId, status: 'running' })
      toast.success(`Agent started`)
    } catch (err: any) {
      updateAgentStatus({ worktreeId, status: 'error', errorMessage: err.message })
      toast.error(`Failed to start agent: ${err.message}`)
    } finally {
      setIsActing(false)
    }
  }

  const stopAgent = async () => {
    if (!session?.sessionId) return
    setIsActing(true)
    try {
      await window.api.agent.stop({ sessionId: session.sessionId })
      updateAgentStatus({ worktreeId, status: 'stopped' })
      toast.success('Agent stopped')
    } catch (err: any) {
      toast.error(`Failed to stop agent: ${err.message}`)
    } finally {
      setIsActing(false)
    }
  }

  const resumeAgent = async () => {
    if (!session?.sessionId) return
    setIsActing(true)
    try {
      await window.api.agent.resume({ sessionId: session.sessionId })
      updateAgentStatus({ worktreeId, status: 'running' })
      toast.success('Agent resumed')
    } catch (err: any) {
      toast.error(`Failed to resume agent: ${err.message}`)
    } finally {
      setIsActing(false)
    }
  }

  const isRunning = session?.status === 'running'
  const isStopped = session?.status === 'stopped'
  const isStarting = session?.status === 'starting'
  const isError = session?.status === 'error'

  return (
    <div className="agent-panel p-3 space-y-3">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-semibold">Agent</h3>
        {session && (
          <AgentStatusBadge status={session.status} />
        )}
      </div>

      {(!session || isStopped || isError) && (
        <div className="flex items-center gap-2">
          <Select value={agentType} onValueChange={v => setAgentType(v as AgentType)}>
            <SelectTrigger className="flex-1 text-sm h-8">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="claude">Claude</SelectItem>
              <SelectItem value="codex">Codex</SelectItem>
              <SelectItem value="custom">Custom</SelectItem>
            </SelectContent>
          </Select>
          <Button size="sm" className="gap-1.5" onClick={startAgent} disabled={isActing || !worktreeId}>
            {isActing ? <Loader2 size={12} className="animate-spin" /> : <Play size={12} />}
            Start
          </Button>
        </div>
      )}

      {isStarting && (
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <Loader2 size={14} className="animate-spin" />
          Starting agent...
        </div>
      )}

      {isRunning && (
        <div className="flex gap-2">
          <Button variant="outline" size="sm" className="flex-1 gap-1.5" onClick={stopAgent} disabled={isActing}>
            <Square size={12} />
            Stop
          </Button>
        </div>
      )}

      {isStopped && session?.sessionId && (
        <Button variant="outline" size="sm" className="w-full gap-1.5" onClick={resumeAgent} disabled={isActing}>
          <RotateCcw size={12} />
          Resume Session
        </Button>
      )}

      {isError && (
        <p className="text-xs text-destructive">Error: {session?.errorMessage}</p>
      )}
    </div>
  )
}

function AgentStatusBadge({ status }: { status: RemoteAgentSession['status'] }) {
  const variants = {
    starting: { label: 'Starting...', className: 'bg-yellow-500/20 text-yellow-600 border-yellow-500/30' },
    running:  { label: 'Running',     className: 'bg-green-500/20  text-green-600  border-green-500/30' },
    stopped:  { label: 'Stopped',     className: 'bg-muted         text-muted-foreground' },
    error:    { label: 'Error',       className: 'bg-red-500/20    text-red-600    border-red-500/30' },
  }
  const { label, className } = variants[status]
  return <Badge variant="outline" className={`text-xs ${className}`}>{label}</Badge>
}
```

---

## Files cần tạo/sửa

| File | Action | Ghi chú |
|------|--------|---------|
| `src/preload/index.ts` | MODIFY | Thêm `agent.start/stop/resume` vào contextBridge |
| `src/renderer/src/web/web-preload-api.ts` | MODIFY | Thêm `agent.*` vào web preload |
| `src/main/ipc/agent-orchestration-ipc.ts` | CREATE | IPC handlers mới |
| `src/main/index.ts` | MODIFY | Register `registerAgentOrchestrationIpc()` |
| `src/renderer/src/hooks/useIpcEvents.ts` | MODIFY | Subscribe `agent:statusChanged` event |
| `src/renderer/src/components/worktree/WorktreeCard.tsx` | MODIFY | Dùng IPC thay vì terminal-based launch |
| `src/renderer/src/store/slices/agent-status.ts` | MODIFY | Thêm `remoteAgentSessions`, `setRemoteAgentSession`, `updateAgentStatus` |
| `src/renderer/src/components/workspace/AgentPanel.tsx` | CREATE | Agent start/stop/resume UI |

---

## Verification

```bash
# 1. Grep verify IPC handler:
grep -r "agent:start" src/main/

# 2. Grep verify preload:
grep -r "agent\.start" src/preload/

# 3. Unit test (Vitest):
# src/main/ipc/__tests__/agent-orchestration-ipc.test.ts
# - agent:start handler calls agentManager.start with correct opts
# - agent:stop handler calls agentManager.stop
# - agent:resume returns resumed:true on success
```

---

## Liên quan

- **BL-AG-01**: Start Agent — IPC bridge ✅ fixed
- **BL-AG-02**: Stop Agent — IPC bridge ✅ fixed  
- **BL-AG-03**: Resume Session — IPC bridge ✅ fixed
- **TDD-FE-07**: §2 useIpcEvents, §15 Key Hook Patterns (Pattern 1)
- **TDD-FE-01**: §2.3 `window.api` abstraction
