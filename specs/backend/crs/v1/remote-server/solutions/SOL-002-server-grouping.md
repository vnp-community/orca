# SOL-002: Server Grouping by Project/Team — Backend Solution

**CR:** [CR-002](../../../../../../../docs/crs/v1/remote-server/CR-002-server-grouping-by-project.md)  
**Backend TDD refs:** `06-persistence.md`, `09-ipc-handlers.md`  
**Depends on:** SOL-001 (schema extension)  
**Effort:** Small (1–2 ngày)  
**Phase:** 1

---

## 1. Phân tích backend hiện tại

Từ `TDD-06 (Persistence)`, SSH targets được persist qua `PersistedState.sshTargets` (JSON array).  
Hiện `listTargets()` trả về flat array, không có grouping.

Từ `TDD-09 (IPC Handlers)`, IPC `ssh.listTargets` trả về tất cả non-runtime targets. Renderer nhận flat array và hiển thị flat list.

**Gap cần giải quyết:** Cần method `listTargetsByProject()` và query API qua IPC.

---

## 2. Giải pháp backend

### 2.1 New query methods trong `SshConnectionStore`

```typescript
// src/main/ssh/ssh-connection-store.ts — ADD METHODS

class SshConnectionStore {
  // ── Existing ────────────────────────────────────────────
  listTargets(): SshTarget[]  // flat list, non-runtime

  // ── NEW: Grouped queries ─────────────────────────────────

  /**
   * Group targets theo project.
   * Returns: Map<project | 'unassigned', SshTarget[]>
   */
  listTargetsByProject(): Map<string, SshTarget[]> {
    const targets = this.listTargets()
    const groups = new Map<string, SshTarget[]>()

    for (const target of targets) {
      const key = target.project ?? 'unassigned'
      const group = groups.get(key) ?? []
      group.push(target)
      groups.set(key, group)
    }

    return groups
  }

  /**
   * Filter targets theo project.
   */
  listTargetsByProjectFilter(project: string): SshTarget[] {
    return this.listTargets().filter(t => t.project === project)
  }

  /**
   * Filter targets theo team.
   */
  listTargetsByTeam(team: string): SshTarget[] {
    return this.listTargets().filter(t => t.team === team)
  }

  /**
   * Filter targets theo environment.
   */
  listTargetsByEnvironment(environment: SshTarget['environment']): SshTarget[] {
    return this.listTargets().filter(t => t.environment === environment)
  }

  /**
   * Get distinct project names in fleet.
   */
  listProjects(): string[] {
    const projects = this.listTargets()
      .map(t => t.project)
      .filter((p): p is string => Boolean(p))
    return [...new Set(projects)].sort()
  }

  /**
   * Get distinct team names in fleet.
   */
  listTeams(): string[] {
    const teams = this.listTargets()
      .map(t => t.team)
      .filter((t): t is string => Boolean(t))
    return [...new Set(teams)].sort()
  }

  /**
   * Combined filter với multiple criteria.
   */
  filterTargets(criteria: {
    project?: string
    team?: string
    environment?: SshTarget['environment']
    tags?: string[]
    search?: string   // fuzzy search on label/host
  }): SshTarget[] {
    return this.listTargets().filter(target => {
      if (criteria.project && target.project !== criteria.project) return false
      if (criteria.team && target.team !== criteria.team) return false
      if (criteria.environment && target.environment !== criteria.environment) return false
      if (criteria.tags?.length) {
        const targetTags = target.tags ?? []
        if (!criteria.tags.some(tag => targetTags.includes(tag))) return false
      }
      if (criteria.search) {
        const q = criteria.search.toLowerCase()
        const haystack = `${target.label} ${target.host}`.toLowerCase()
        if (!haystack.includes(q)) return false
      }
      return true
    })
  }
}
```

### 2.2 IPC handlers mới

```typescript
// src/main/ipc/ssh.ts — ADD HANDLERS

// Handler: ssh.listTargetsByProject
// Returns: Record<project | 'unassigned', SshTarget[]>
ipcMain.handle('ssh:listTargetsByProject', () => {
  const grouped = sshConnectionStore.listTargetsByProject()
  // Convert Map → plain object for IPC serialization
  return Object.fromEntries(grouped.entries())
})

// Handler: ssh.listProjects
// Returns: string[]
ipcMain.handle('ssh:listProjects', () => {
  return sshConnectionStore.listProjects()
})

// Handler: ssh.listTeams
// Returns: string[]
ipcMain.handle('ssh:listTeams', () => {
  return sshConnectionStore.listTeams()
})

// Handler: ssh.filterTargets
// Returns: SshTarget[]
ipcMain.handle('ssh:filterTargets', (_event, criteria: FilterCriteria) => {
  return sshConnectionStore.filterTargets(criteria)
})
```

### 2.3 RuntimeSyncWindowGraph extension

Từ `TDD-07 (Runtime Service)`, `buildRuntimeSyncWindowGraph()` serialize state cho frontend sync.  
Thêm fleet metadata vào graph:

```typescript
// src/main/runtime/orca-runtime.ts — MODIFY buildRuntimeSyncWindowGraph()

function buildRuntimeSyncWindowGraph(): RuntimeSyncWindowGraph {
  return {
    // ... existing fields ...
    sshTargets: sshConnectionStore.listTargets(),  // đã có
    // NEW: add fleet metadata
    sshProjects: sshConnectionStore.listProjects(),
    sshTeams: sshConnectionStore.listTeams(),
  }
}
```

### 2.4 Frontend-facing grouped response type

```typescript
// src/shared/ssh-types.ts — ADD TYPES

/** Grouped SSH targets for sidebar display */
export type SshTargetGroup = {
  key: string              // project name | 'unassigned'
  label: string            // Display label
  targets: SshTarget[]
  isUnassigned: boolean
}

/** Convert flat list to groups (can run in renderer too) */
export function groupSshTargetsByProject(targets: SshTarget[]): SshTargetGroup[] {
  const map = new Map<string, SshTarget[]>()

  for (const target of targets) {
    const key = target.project ?? '__unassigned__'
    const group = map.get(key) ?? []
    group.push(target)
    map.set(key, group)
  }

  const groups: SshTargetGroup[] = []

  // Named projects first (sorted)
  const projectKeys = [...map.keys()]
    .filter(k => k !== '__unassigned__')
    .sort()

  for (const key of projectKeys) {
    groups.push({
      key,
      label: key,
      targets: map.get(key)!,
      isUnassigned: false,
    })
  }

  // Unassigned last
  const unassigned = map.get('__unassigned__')
  if (unassigned?.length) {
    groups.push({
      key: '__unassigned__',
      label: 'Unassigned',
      targets: unassigned,
      isUnassigned: true,
    })
  }

  return groups
}
```

---

## 3. Persistence — không cần migration

Từ `TDD-06`, PersistedState dùng JSON store.  
Các field `project`, `team`, `environment` đã được thêm trong SOL-001 (optional) → backward compatible.

Nếu migration SQLite thực sự cần (nếu có switch sang SQL trong tương lai):

```sql
-- Dự phòng — chỉ dùng nếu store backend đổi sang SQLite full
ALTER TABLE sshTargets ADD COLUMN project TEXT;
ALTER TABLE sshTargets ADD COLUMN team TEXT;
ALTER TABLE sshTargets ADD COLUMN environment TEXT CHECK(environment IN ('development','staging','production'));
ALTER TABLE sshTargets ADD COLUMN tags TEXT;        -- JSON array: '["golang","backend"]'
ALTER TABLE sshTargets ADD COLUMN repos TEXT;       -- JSON array
ALTER TABLE sshTargets ADD COLUMN fleet_id TEXT;    -- fleet config stable ID
ALTER TABLE sshTargets ADD COLUMN fleet_config_source TEXT;  -- path to yaml file

CREATE INDEX IF NOT EXISTS idx_ssh_targets_project ON sshTargets(project);
CREATE INDEX IF NOT EXISTS idx_ssh_targets_team ON sshTargets(team);
CREATE INDEX IF NOT EXISTS idx_ssh_targets_fleet_id ON sshTargets(fleet_id);
```

---

## 4. Zustand store slice cập nhật (Renderer side)

Thuộc frontend, nhưng cần backend expose đúng data shape:

```typescript
// src/renderer/src/store/slices/ssh.ts (hiện có)
// Thêm:
type SshSlice = {
  sshTargets: SshTarget[]          // flat list (existing)
  sshTargetsByProject: Record<string, SshTarget[]>  // NEW: grouped
  sshProjects: string[]            // NEW: distinct project names
  sshTeams: string[]               // NEW: distinct team names
}
```

---

## 5. Files cần thay đổi

| File | Action | Chi tiết |
|------|--------|---------|
| `src/main/ssh/ssh-connection-store.ts` | MODIFY | 5 query methods mới |
| `src/main/ipc/ssh.ts` | MODIFY | 4 IPC handlers mới |
| `src/shared/ssh-types.ts` | MODIFY | `SshTargetGroup` type + `groupSshTargetsByProject()` |
| `src/main/runtime/orca-runtime.ts` | MODIFY | Thêm `sshProjects`, `sshTeams` vào sync graph |

---

## 6. Testing

```typescript
// src/main/ssh/ssh-connection-store.test.ts — ADD TESTS

describe('fleet grouping', () => {
  it('groups targets by project', () => {
    store.addTarget({ ...baseTarget, id: 't1', project: 'vnp-blc' })
    store.addTarget({ ...baseTarget, id: 't2', project: 'vnp-blc' })
    store.addTarget({ ...baseTarget, id: 't3', project: 'vnp-ai-ops' })
    store.addTarget({ ...baseTarget, id: 't4' })  // unassigned

    const groups = store.listTargetsByProject()
    expect(groups.get('vnp-blc')).toHaveLength(2)
    expect(groups.get('vnp-ai-ops')).toHaveLength(1)
    expect(groups.get('unassigned')).toHaveLength(1)
  })

  it('filterTargets combines criteria', () => {
    const results = store.filterTargets({
      project: 'vnp-blc',
      environment: 'development',
    })
    expect(results.every(t => t.project === 'vnp-blc')).toBe(true)
    expect(results.every(t => t.environment === 'development')).toBe(true)
  })
})
```

---

## 7. Implementation Status

> **✅ IMPLEMENTED — Phase 1 Complete**  
> Ngày: 2026-07-22

### Đã triển khai

| File | Status | Chi tiết |
|------|--------|---------|
| [`src/main/ssh/ssh-connection-store.ts`](../../../../../src/main/ssh/ssh-connection-store.ts) | ✅ Done | `listTargetsByProject()`, `listTeams()`, `filterTargets()`, `listProjects()`, `groupSshTargets()` |
| [`src/main/ipc/ssh.ts`](../../../../../src/main/ipc/ssh.ts) | ✅ Done | `ssh:listTargetsByProject`, `ssh:listProjects`, `ssh:listTeams`, `ssh:filterTargets`, `ssh:getAllConnectionStates` |
| [`src/shared/ssh-types.ts`](../../../../../src/shared/ssh-types.ts) | ✅ Done | `SshTargetGroup` type + `groupSshTargetsByProject()` pure function |
| [`src/main/runtime/rpc/methods/ssh.ts`](../../../../../src/main/runtime/rpc/methods/ssh.ts) | ✅ Done | `ssh.listProjects`, `ssh.listTeams`, `ssh.filterTargets`, `ssh.getAllConnectionStates` RPC methods |

### Deviation từ design gốc

> **Note:** `RuntimeSyncWindowGraph` không được thêm `sshProjects`/`sshTeams` — graph này flow renderer→main (không phải ngược lại). Thay bằng RPC methods `ssh.listProjects`/`ssh.listTeams` — đúng kiến trúc hơn.

### Notes

- **TASK-005** (ssh-store grouping queries): ✅ Done
- **TASK-006/007/008** (grouping IPC handlers, types, RPC): ✅ Done
