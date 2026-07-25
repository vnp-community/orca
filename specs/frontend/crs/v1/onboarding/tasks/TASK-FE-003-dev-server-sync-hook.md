# TASK-FE-003: Tạo useDevServersSync hook + IPC subscription

> **Status: ✅ COMPLETED** — 2026-07-23
> **Files created/modified:**
> - `src/renderer/src/hooks/useDevServersSync.ts` [NEW]
> - `src/renderer/src/hooks/useIpcEvents.ts` [MODIFY] — import + call useDevServersSync()

**Phase:** 1 | **Solution:** [FE-SOL-A](../solutions/FE-SOL-A-dev-server-ui.md) | **CR:** CR-OB-002  
**Depends on:** TASK-FE-001, TASK-FE-002, TASK-FE-007  
**Estimated effort:** ~30 phút

---

## Context

Đọc trước:
- [`src/renderer/src/hooks/useIpcEvents.ts`](../../../../../src/renderer/src/hooks/useIpcEvents.ts) — pattern IPC subscription + cleanup
- [`../solutions/FE-SOL-A-dev-server-ui.md`](../solutions/FE-SOL-A-dev-server-ui.md) — Section 3 (Hooks)

---

## Goal

Tạo `useDevServersSync` hook để load danh sách dev servers lúc khởi động và lắng nghe status change events từ backend. Đăng ký hook vào `useIpcEvents`.

---

## Steps

1. **Tạo** `src/renderer/src/hooks/useDevServersSync.ts`:

```typescript
import { useEffect } from 'react'
import { useAppStore } from '../store'

export function useDevServersSync(): void {
  const setDevServers = useAppStore((s) => s.setDevServers)
  const upsertDevServer = useAppStore((s) => s.upsertDevServer)
  const updateDevServerStatus = useAppStore((s) => s.updateDevServerStatus)
  const setActiveDevServerId = useAppStore((s) => s.setActiveDevServerId)

  useEffect(() => {
    // Load initial list
    void window.api.devServer.list().then((servers) => {
      setDevServers(servers)
    })

    // Load active dev server from settings
    void window.api.settings.getGlobalSettings().then((settings) => {
      if (settings?.activeDevServerId) {
        setActiveDevServerId(settings.activeDevServerId)
      }
    })

    // Subscribe to status changes (push events from backend)
    const offStatus = window.api.devServer.onStatusChanged((event) => {
      updateDevServerStatus(event.id, event.status, {
        platform: event.platform ?? undefined,
        lastError: event.error ?? null,
        lastConnectedAt: event.status === 'connected' ? Date.now() : undefined,
      })
    })

    return () => {
      offStatus()
    }
  }, [setDevServers, updateDevServerStatus, setActiveDevServerId])
}
```

2. **Sửa** `src/renderer/src/hooks/useIpcEvents.ts` — thêm `useDevServersSync()` call:

```typescript
// Trong body của useIpcEvents():
useDevServersSync()
```

> **Lưu ý:** Pattern đúng là gọi hook con bên trong `useIpcEvents` hoặc gọi `useDevServersSync` trực tiếp tại root component — kiểm tra pattern hiện tại để quyết định vị trí phù hợp.

3. **Viết test** `src/renderer/src/hooks/__tests__/useDevServersSync.test.ts`:

```typescript
// @vitest-environment happy-dom
describe('useDevServersSync', () => {
  it('gọi window.api.devServer.list() khi mount')
  it('setDevServers() được gọi với kết quả từ API')
  it('subscribe window.api.devServer.onStatusChanged')
  it('updateDevServerStatus() được gọi khi status event arrive')
  it('cleanup: offStatus() được gọi khi unmount')
  it('không gọi window.api.devServer.list() lại khi re-render')
})
```

---

## Acceptance Criteria

- [ ] `useDevServersSync` load list khi mount
- [ ] `onStatusChanged` được subscribe và cleanup khi unmount
- [ ] Tests pass

## Output Files

- **[NEW]** `src/renderer/src/hooks/useDevServersSync.ts`
- **[NEW]** `src/renderer/src/hooks/__tests__/useDevServersSync.test.ts`
- **[MODIFY]** `src/renderer/src/hooks/useIpcEvents.ts`
