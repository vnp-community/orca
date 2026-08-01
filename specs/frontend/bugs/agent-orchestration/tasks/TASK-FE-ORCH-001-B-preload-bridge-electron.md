# TASK-FE-ORCH-001-B: Preload Bridge — contextBridge cho Electron mode

**Domain:** agent-orchestration  
**Solution Ref:** SOL-FE-ORCH-001 Bước 2  
**Priority:** 🔴 P0  
**Estimated:** 20 phút  
**Status:** ✅ DONE — Implemented in preload/index.ts (agentOrchestration namespace)

---

## Mục tiêu

Expose `agent.start/stop/resume/onStatusChanged/offStatusChanged` vào Electron Renderer qua `contextBridge`.

---

## Files cần sửa

- `src/preload/index.ts`

---

## Các bước thực thi

Tìm phần `contextBridge.exposeInMainWorld('api', { ... })` trong `src/preload/index.ts`, thêm block sau vào object đó:

```typescript
agent: {
  start:  (opts: AgentStartOptions)  => ipcRenderer.invoke('agent:start',  opts),
  stop:   (opts: AgentStopOptions)   => ipcRenderer.invoke('agent:stop',   opts),
  resume: (opts: AgentResumeOptions) => ipcRenderer.invoke('agent:resume', opts),
  onStatusChanged: (cb: (event: AgentStatusEvent) => void) =>
    ipcRenderer.on('agent:statusChanged', (_evt, e) => cb(e)),
  offStatusChanged: (cb: (event: AgentStatusEvent) => void) =>
    ipcRenderer.removeListener('agent:statusChanged', cb),
},
```

Import types từ shared file nếu cần.

---

## Verify

```bash
grep -n "agent:start" src/preload/index.ts
```

## Depends on
TASK-FE-ORCH-001-A (OrcaApi interface)

## Blocking
TASK-FE-ORCH-001-C
