# TASK-FE-002: Tạo Zustand dev-servers slice

> **Status: ✅ COMPLETED** — 2026-07-23
> **Files created/modified:**
> - `src/renderer/src/store/slices/dev-servers.ts` [NEW]
> - `src/renderer/src/store/slices/dev-servers.test.ts` [NEW]
> - `src/renderer/src/store/index.ts` [MODIFY] — register slice
> - `src/renderer/src/store/types.ts` [MODIFY] — add to AppState

**Phase:** 1 | **Solution:** [FE-SOL-A](../solutions/FE-SOL-A-dev-server-ui.md) | **CR:** CR-OB-002  
**Depends on:** TASK-FE-001  
**Estimated effort:** ~45 phút

---

## Context

Đọc trước:
- [`src/renderer/src/store/index.ts`](../../../../../src/renderer/src/store/index.ts) — cách register slices
- [`src/renderer/src/store/slices/ssh.ts`](../../../../../src/renderer/src/store/slices/ssh.ts) — ví dụ slice đơn giản để xem pattern
- [`../solutions/FE-SOL-A-dev-server-ui.md`](../solutions/FE-SOL-A-dev-server-ui.md) — Section 2

---

## Goal

Tạo Zustand slice `dev-servers` quản lý toàn bộ state liên quan đến DevServer, đăng ký vào store chính.

---

## Steps

1. **Tạo** `src/renderer/src/store/slices/dev-servers.ts`:

```typescript
import { useShallow } from 'zustand/react/shallow'
import type { DevServer } from '../../../../shared/dev-server-types'
import type { AppState, SetState } from '../index'

export type DevServerSlice = {
  devServers: DevServer[]
  activeDevServerId: string | null
  setDevServers: (servers: DevServer[]) => void
  upsertDevServer: (server: DevServer) => void
  removeDevServer: (id: string) => void
  setActiveDevServerId: (id: string | null) => void
  updateDevServerStatus: (
    id: string,
    status: DevServer['status'],
    extra?: Partial<Pick<DevServer, 'platform' | 'arch' | 'nodeVersion' | 'lastConnectedAt' | 'lastError'>>
  ) => void
}

export const createDevServerSlice = (set: SetState<AppState>): DevServerSlice => ({
  devServers: [],
  activeDevServerId: null,

  setDevServers: (servers) => set({ devServers: servers }),

  upsertDevServer: (server) =>
    set((state) => ({
      devServers: state.devServers.some((ds) => ds.id === server.id)
        ? state.devServers.map((ds) => (ds.id === server.id ? { ...ds, ...server } : ds))
        : [...state.devServers, server],
    })),

  removeDevServer: (id) =>
    set((state) => ({
      devServers: state.devServers.filter((ds) => ds.id !== id),
      activeDevServerId: state.activeDevServerId === id ? null : state.activeDevServerId,
    })),

  setActiveDevServerId: (id) => set({ activeDevServerId: id }),

  updateDevServerStatus: (id, status, extra = {}) =>
    set((state) => ({
      devServers: state.devServers.map((ds) =>
        ds.id === id ? { ...ds, status, ...extra } : ds
      ),
    })),
})
```

2. **Thêm** `createDevServerSlice` vào `src/renderer/src/store/index.ts`:
   - Import `createDevServerSlice` và `DevServerSlice`
   - Thêm `DevServerSlice` vào `AppState` type
   - Gọi `...createDevServerSlice(...a)` trong `create<AppState>()`

3. **Tạo** selector hooks (thêm vào cuối file `dev-servers.ts`):

```typescript
import { useAppStore } from '../index'

export function useDevServers() {
  return useAppStore(useShallow((s) => s.devServers))
}

export function useActiveDevServer() {
  return useAppStore(
    useShallow((s) => {
      const id = s.activeDevServerId
      return id ? s.devServers.find((ds) => ds.id === id) ?? null : null
    })
  )
}

export function useConnectedDevServers() {
  return useAppStore(useShallow((s) => s.devServers.filter((ds) => ds.status === 'connected')))
}

export function useDevServerById(id: string | null) {
  return useAppStore((s) => (id ? s.devServers.find((ds) => ds.id === id) ?? null : null))
}
```

4. **Viết test** `src/renderer/src/store/slices/__tests__/dev-servers.test.ts`:

```typescript
import { describe, it, expect, beforeEach } from 'vitest'

describe('dev-servers slice', () => {
  // Tạo isolated store cho mỗi test
  let store: ReturnType<typeof createTestStore>
  beforeEach(() => { store = createTestStore() })

  it('setDevServers() thay thế toàn bộ list')
  it('upsertDevServer() thêm server mới khi id chưa có')
  it('upsertDevServer() cập nhật server khi id đã có')
  it('removeDevServer() xóa server và giữ nguyên activeDevServerId nếu khác id')
  it('removeDevServer() reset activeDevServerId = null khi xóa active server')
  it('setActiveDevServerId() cập nhật activeDevServerId')
  it('updateDevServerStatus() chỉ thay đổi đúng server, giữ nguyên các server khác')
  it('updateDevServerStatus() merge extra fields vào server')
})
```

5. **Verify**: `pnpm tsc --noEmit` && `pnpm test src/renderer/src/store/slices/__tests__/dev-servers.test.ts`

---

## Acceptance Criteria

- [ ] `createDevServerSlice` export đúng type `DevServerSlice`
- [ ] Đã đăng ký vào `store/index.ts`
- [ ] `useDevServers()`, `useActiveDevServer()`, `useConnectedDevServers()` hoạt động
- [ ] `removeDevServer()` reset `activeDevServerId` khi xóa active server
- [ ] Tests pass

## Output Files

- **[NEW]** `src/renderer/src/store/slices/dev-servers.ts`
- **[NEW]** `src/renderer/src/store/slices/__tests__/dev-servers.test.ts`
- **[MODIFY]** `src/renderer/src/store/index.ts`
