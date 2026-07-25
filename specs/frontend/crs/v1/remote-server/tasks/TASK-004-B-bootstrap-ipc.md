# TASK-004-B — Bootstrap IPC Events + Preload API

**Task ID:** TASK-004-B  
**CR:** CR-004 — Dev Server Bootstrap Automation  
**Solution Ref:** SOL-CR-004, Section 3  
**Dependencies:** TASK-004-A  
**Estimated:** 1 giờ  
**Status:** ✅ DONE

---

## Mục tiêu

Expose bootstrap API qua `window.api.ssh` và handle streaming bootstrap events trong `useIpcEvents`.

---

## Bước thực thi

### Bước 1: Thêm API vào preload

```typescript
// Thêm vào ssh namespace trong preload/index.ts:
bootstrapServer: (args: {
  serverId: string
  options?: {
    installNode?: boolean
    installGit?: boolean
    cloneRepos?: boolean
  }
}) => ipcRenderer.invoke('ssh:bootstrapServer', args),

cancelBootstrap: (serverId: string) =>
  ipcRenderer.invoke('ssh:cancelBootstrap', serverId),

onBootstrapProgress: (
  callback: (event: BootstrapProgressEvent) => void
) => {
  const handler = (_: IpcRendererEvent, event: BootstrapProgressEvent) =>
    callback(event)
  ipcRenderer.on('ssh:bootstrapProgress', handler)
  return () => ipcRenderer.removeListener('ssh:bootstrapProgress', handler)
},
```

### Bước 2: Định nghĩa BootstrapProgressEvent type

```typescript
export type BootstrapProgressEvent =
  | { type: 'bootstrap.started'; serverId: string; serverLabel: string }
  | { type: 'bootstrap.step.started'; serverId: string; stepId: string }
  | { type: 'bootstrap.step.done'; serverId: string; stepId: string; detail?: string }
  | { type: 'bootstrap.step.error'; serverId: string; stepId: string; error: string }
  | { type: 'bootstrap.step.skipped'; serverId: string; stepId: string; reason: string }
  | { type: 'bootstrap.log'; serverId: string; line: string }
  | { type: 'bootstrap.done'; serverId: string; serverLabel: string }
  | { type: 'bootstrap.error'; serverId: string; serverLabel: string; error: string }
```

### Bước 3: Thêm handlers trong useIpcEvents.ts

```typescript
// [NEW CR-004] Bootstrap events
const unsubBootstrap = window.api.ssh.onBootstrapProgress?.((event) => {
  const store = useAppStore.getState()

  switch (event.type) {
    case 'bootstrap.started':
      store.initBootstrap(event.serverId)
      break
    case 'bootstrap.step.started':
      store.updateBootstrapStep(event.serverId, event.stepId, {
        status: 'running',
      })
      break
    case 'bootstrap.step.done':
      store.updateBootstrapStep(event.serverId, event.stepId, {
        status: 'done',
        detail: event.detail ?? null,
      })
      break
    case 'bootstrap.step.error':
      store.updateBootstrapStep(event.serverId, event.stepId, {
        status: 'error',
        error: event.error,
      })
      break
    case 'bootstrap.step.skipped':
      store.updateBootstrapStep(event.serverId, event.stepId, {
        status: 'skipped',
        detail: event.reason,
      })
      break
    case 'bootstrap.log':
      store.appendBootstrapLog(event.serverId, event.line)
      break
    case 'bootstrap.done':
      store.finishBootstrap(event.serverId, true)
      toast.success(
        translate('fleet.bootstrap.done', `Bootstrap complete: ${event.serverLabel}`)
      )
      scheduleRuntimeGraphSync()
      break
    case 'bootstrap.error':
      store.finishBootstrap(event.serverId, false)
      toast.error(
        translate('fleet.bootstrap.failed', `Bootstrap failed: ${event.serverLabel}`)
      )
      break
  }
})

// Cleanup:
unsubBootstrap?.()
```

### Bước 4: Verify

```bash
npx tsc --noEmit 2>&1 | grep "bootstrap\|Bootstrap" | head -10
```

---

## Acceptance Criteria

- [x] `window.api.ssh.bootstrapServer(args)` callable
- [x] `window.api.ssh.cancelBootstrap(serverId)` callable
- [x] `window.api.ssh.onBootstrapProgress(cb)` registrable
- [x] 8 event types đều có handler trong useIpcEvents
- [x] store được update đúng sau mỗi event
- [x] Toast hiển thị khi done/error

---

## Implementation Notes

> **Completed:** 2026-07-23 | `preload/api-types.ts`: bootstrapServer, cancelBootstrap, onBootstrapProgress. `preload/index.ts`: IPC bridges. `useIpcEvents.ts`: 6 event types (started/step-update/log/done/error/cancelled) + default:break + toast on done/error. TypeScript: ✅ 0 errors.
