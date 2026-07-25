# TASK-003: Tạo file `src/main/dev-server/dev-server-store.ts`

**Phase:** 1 — Foundation  
**Solution:** [SOL-002](../solutions/SOL-002-dev-server-manager.md) §4  
**Depends on:** TASK-001, TASK-002  
**Blocks:** TASK-004

---

## Mục tiêu

Tạo class `DevServerStore` thực hiện CRUD operations trên `PersistedState.devServers` thông qua `Store`.

---

## File cần tạo

**Path:** `src/main/dev-server/dev-server-store.ts`

---

## Nội dung cần implement

```typescript
import type { Store } from '../persistence'
import type { DevServerInput, PersistedDevServer } from '../../shared/dev-server-types'
import { randomUUID } from 'node:crypto'

export class DevServerStore {
  constructor(private store: Store) {}

  list(): PersistedDevServer[] {
    return this.store.getState().devServers ?? []
  }

  add(input: DevServerInput): PersistedDevServer {
    const record: PersistedDevServer = {
      id: `ds-${randomUUID()}`,
      name: input.name,
      connectionType: input.connectionType,
      sshTargetId: input.sshTargetId,
      wsUrl: input.wsUrl,
      workspaceDir: null,
      addedAt: Date.now()
    }
    this.store.mutate(state => {
      state.devServers = [...(state.devServers ?? []), record]
    })
    return record
  }

  update(id: string, updates: Partial<PersistedDevServer>): void {
    this.store.mutate(state => {
      const idx = state.devServers.findIndex(ds => ds.id === id)
      if (idx >= 0) {
        state.devServers[idx] = { ...state.devServers[idx], ...updates }
      }
    })
  }

  remove(id: string): void {
    this.store.mutate(state => {
      state.devServers = state.devServers.filter(ds => ds.id !== id)
    })
  }
}
```

---

## Acceptance Criteria

- [x] File tồn tại tại `src/main/dev-server/dev-server-store.ts`
- [x] Class `DevServerStore` được export
- [x] `add()` tạo id với prefix `ds-` và uuid
- [x] `add()` set `workspaceDir: null` và `addedAt: Date.now()`
- [x] `list()` trả về `[]` nếu `state.devServers` là `undefined`
- [x] `update()` chỉ cập nhật record đúng id, không ảnh hưởng records khác
- [x] `remove()` lọc đúng record theo id
- [x] TypeScript compile thành công
