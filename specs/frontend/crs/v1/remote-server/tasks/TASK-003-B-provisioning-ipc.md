# TASK-003-B — Provisioning IPC Events + Preload API

**Task ID:** TASK-003-B  
**CR:** CR-003 — Bulk Server Provisioning  
**Solution Ref:** SOL-CR-003, Section 3  
**Dependencies:** TASK-003-A  
**Estimated:** 1–2 giờ  
**Status:** ✅ DONE

---

## Mục tiêu

Expose provisioning API qua `window.api.ssh` và thêm event handlers trong `useIpcEvents`.

---

## Bước thực thi

### Bước 1: Thêm API vào preload

Trong `src/preload/index.ts` và `src/renderer/src/web/web-preload-api.ts`:

```typescript
// Thêm vào ssh namespace:
provisionFleetServers: (args: {
  serverIds: string[]
  concurrency?: number
}) => ipcRenderer.invoke('ssh:provisionFleet', args),

cancelProvisioning: () =>
  ipcRenderer.invoke('ssh:cancelProvisioning'),

onProvisioningProgress: (
  callback: (event: ProvisioningProgressEvent) => void
) => {
  const handler = (_: IpcRendererEvent, event: ProvisioningProgressEvent) =>
    callback(event)
  ipcRenderer.on('ssh:provisioningProgress', handler)
  return () => ipcRenderer.removeListener('ssh:provisioningProgress', handler)
},
```

### Bước 2: Định nghĩa ProvisioningProgressEvent type

```typescript
// Trong shared types hoặc store/types.ts:
export type ProvisioningProgressEvent =
  | { type: 'server.started'; serverId: string }
  | { type: 'server.relay-deploying'; serverId: string }
  | { type: 'server.done'; serverId: string; relayVersion: string }
  | { type: 'server.error'; serverId: string; error: string }
  | { type: 'server.skipped'; serverId: string; reason: string }
  | { type: 'session.done'; totalDone: number; totalFailed: number }
```

### Bước 3: Thêm handlers trong useIpcEvents.ts

```typescript
// [NEW CR-003] Provisioning events
const unsubProvisioning = window.api.ssh.onProvisioningProgress?.((event) => {
  const store = useAppStore.getState()
  
  switch (event.type) {
    case 'server.started':
      store.updateProvisioningServerStatus(event.serverId, {
        status: 'connecting',
        startedAt: Date.now(),
      })
      break
    case 'server.relay-deploying':
      store.updateProvisioningServerStatus(event.serverId, {
        status: 'deploying-relay',
      })
      break
    case 'server.done':
      store.updateProvisioningServerStatus(event.serverId, {
        status: 'done',
        completedAt: Date.now(),
        relayVersion: event.relayVersion,
      })
      scheduleRuntimeGraphSync()
      break
    case 'server.error':
      store.updateProvisioningServerStatus(event.serverId, {
        status: 'error',
        error: event.error,
        completedAt: Date.now(),
      })
      break
    case 'server.skipped':
      store.updateProvisioningServerStatus(event.serverId, {
        status: 'skipped',
      })
      break
    case 'session.done':
      store.finishProvisioningSession()
      const session = useAppStore.getState().provisioningSession
      if (session) {
        toast.success(
          translate(
            'fleet.provision.summary',
            `Provisioning complete: ${event.totalDone} ready, ${event.totalFailed} failed`
          )
        )
      }
      break
  }
})

// Cleanup:
unsubProvisioning?.()
```

### Bước 4: Verify

```bash
npx tsc --noEmit 2>&1 | grep "provisioning\|Provisioning" | head -10
```

---

## Acceptance Criteria

- [x] `window.api.ssh.provisionFleetServers({ serverIds })` callable
- [x] `window.api.ssh.cancelProvisioning()` callable
- [x] `window.api.ssh.onProvisioningProgress(cb)` registrable
- [x] useIpcEvents xử lý tất cả 6 event types
- [x] Store được update đúng sau mỗi event
- [x] Toast summary khi session.done

---

## Notes cho AI

- Event types liệt kê đầy đủ, mỗi case phải có handler (không bỏ `default`)
- `useAppStore.getState()` dùng ngoài React hooks là OK (trong event handlers)
- Cẩn thận stale closure: đọc store state fresh trong event handler

---

## Implementation Notes

> **Completed:** 2026-07-23 | `preload/api-types.ts`: provisionFleetServers, cancelProvisioning, onProvisioningProgress. `preload/index.ts`: IPC bridges. `useIpcEvents.ts`: 7 event types all handled + default:break + toast on session done. TypeScript: ✅ 0 errors.
