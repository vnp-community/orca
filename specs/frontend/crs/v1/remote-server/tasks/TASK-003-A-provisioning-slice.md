# TASK-003-A — Tạo ProvisioningSlice

**Task ID:** TASK-003-A  
**CR:** CR-003 — Bulk Server Provisioning  
**Solution Ref:** SOL-CR-003, Section 2  
**Dependencies:** TASK-001-A, TASK-002-A  
**Estimated:** 2 giờ  
**Status:** ✅ DONE

---

## Mục tiêu

Tạo Zustand slice mới `provisioningSlice` để quản lý trạng thái bulk provisioning session.

---

## File cần tạo

`src/renderer/src/store/slices/provisioning.ts`

---

## Bước thực thi

### Bước 1: Tạo file slice mới

```typescript
// src/renderer/src/store/slices/provisioning.ts
import type { StateCreator } from 'zustand'
import type { AppState } from '@/store/types'

// ─── Types ────────────────────────────────────────────────────────────────────

export type ProvisioningServerStatus =
  | 'pending'
  | 'connecting'
  | 'deploying-relay'
  | 'done'
  | 'error'
  | 'skipped'

export type ProvisioningServerEntry = {
  serverId: string
  label: string
  host: string
  status: ProvisioningServerStatus
  error: string | null
  startedAt: number | null
  completedAt: number | null
  relayVersion: string | null
}

export type ProvisioningSessionPhase =
  | 'running'
  | 'done'
  | 'cancelled'

export type ProvisioningSession = {
  sessionId: string
  startedAt: number
  phase: ProvisioningSessionPhase
  servers: ProvisioningServerEntry[]
  concurrency: number
}

export type ProvisioningSlice = {
  provisioningSession: ProvisioningSession | null

  startProvisioningSession: (serverIds: string[]) => void
  updateProvisioningServerStatus: (
    serverId: string,
    update: Partial<ProvisioningServerEntry>
  ) => void
  finishProvisioningSession: () => void
  cancelProvisioningSession: () => void
}

// ─── Slice Factory ────────────────────────────────────────────────────────────

export const createProvisioningSlice: StateCreator<
  AppState,
  [],
  [],
  ProvisioningSlice
> = (set, get) => ({
  provisioningSession: null,

  startProvisioningSession: (serverIds) => {
    const targets = get().sshTargets ?? []
    set((s) => {
      s.provisioningSession = {
        sessionId: crypto.randomUUID(),
        startedAt: Date.now(),
        phase: 'running',
        concurrency: 3,
        servers: serverIds.map((id) => {
          const target = targets.find((t) => t.id === id)
          return {
            serverId: id,
            label: target?.label ?? id,
            host: target?.host ?? '',
            status: 'pending' as ProvisioningServerStatus,
            error: null,
            startedAt: null,
            completedAt: null,
            relayVersion: null,
          }
        }),
      }
    })
  },

  updateProvisioningServerStatus: (serverId, update) =>
    set((s) => {
      const session = s.provisioningSession
      if (!session) return
      const entry = session.servers.find((e) => e.serverId === serverId)
      if (entry) Object.assign(entry, update)
    }),

  finishProvisioningSession: () =>
    set((s) => {
      if (s.provisioningSession) {
        s.provisioningSession.phase = 'done'
      }
    }),

  cancelProvisioningSession: () =>
    set((s) => {
      s.provisioningSession = null
    }),
})
```

### Bước 2: Đăng ký vào AppState

Trong `src/renderer/src/store/types.ts`, thêm `ProvisioningSlice` vào `AppState`:

```typescript
import type { ProvisioningSlice } from './slices/provisioning'
// Trong AppState type intersection: & ProvisioningSlice
```

### Bước 3: Đăng ký vào store index

Trong `src/renderer/src/store/index.ts`:

```typescript
import { createProvisioningSlice } from './slices/provisioning'
// Trong useAppStore create: ...createProvisioningSlice(...a),
```

### Bước 4: Verify

```bash
npx tsc --noEmit 2>&1 | grep "provisioning\|ProvisioningSlice" | head -10
```

---

## Acceptance Criteria

- [x] File `provisioning.ts` được tạo với đầy đủ types và slice
- [x] `AppState` có `ProvisioningSlice` (không compile error)
- [x] `useAppStore.getState().provisioningSession` accessible
- [x] `startProvisioningSession(['id1', 'id2'])` tạo session với servers list
- [x] `updateProvisioningServerStatus('id', { status: 'done' })` update đúng entry
- [x] `finishProvisioningSession()` set phase = 'done'
- [x] `cancelProvisioningSession()` set session = null

---

## Notes cho AI

- Zustand với immer middleware: dùng direct mutation trong `set(s => { s.x = y })`
- `crypto.randomUUID()` — có sẵn trong modern browsers và Node
- `get()` để đọc current state trong actions (không dùng `useAppStore.getState()` trong slice)
- Cẩn thận import cycle: không import component từ slice

---

## Implementation Notes

> **Completed:** 2026-07-23 | `store/slices/provisioning.ts`: ProvisioningSession, ProvisioningServerStatus, ProvisioningSlice. startProvisioningSession() builds server list, updateProvisioningServerStatus() updates entry, finishProvisioningSession() phase=done, cancelProvisioningSession() session=null. store/types.ts + store/index.ts: registered. TypeScript: ✅ 0 errors.
