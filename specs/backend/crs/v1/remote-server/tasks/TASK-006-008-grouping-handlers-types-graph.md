# TASK-006: Thêm IPC Handlers — listTargetsByProject, listProjects, filterTargets

**Source:** SOL-002  
**Phase:** 1 | **Effort:** XS (<30 min)  
**Depends on:** TASK-005

---

## Objective

Expose các grouping query methods qua IPC để renderer có thể gọi.

---

## File to modify

**`src/main/ipc/ssh.ts`**

---

## Implementation

```typescript
  // ── Fleet Grouping IPC Handlers (NEW) ──────────────────────

  ipcMain.handle('ssh:listTargetsByProject', () => {
    const grouped = sshConnectionStore.listTargetsByProject()
    // Convert Map → plain object for IPC serialization
    return Object.fromEntries(grouped.entries())
  })

  ipcMain.handle('ssh:listProjects', () => {
    return sshConnectionStore.listProjects()
  })

  ipcMain.handle('ssh:listTeams', () => {
    return sshConnectionStore.listTeams()
  })

  ipcMain.handle('ssh:filterTargets', (_event, criteria: SshTargetFilterCriteria) => {
    return sshConnectionStore.filterTargets(criteria)
  })

  ipcMain.handle('ssh:getAllConnectionStates', () => {
    // Return a map of targetId → SshConnectionState for all targets
    const targets = sshConnectionStore.listTargets()
    const states: Record<string, SshConnectionState | null> = {}
    for (const target of targets) {
      states[target.id] = sshManager.getConnectionState(target.id) ?? null
    }
    return states
  })
```

### Add import if needed

```typescript
import type { SshTargetFilterCriteria } from '../ssh/ssh-connection-store'
import type { SshConnectionState } from '../../../shared/ssh-types'
```

---

## Done criteria

- [x] `ssh:listTargetsByProject` handler returns `Record<string, SshTarget[]>`
- [x] `ssh:listProjects` handler returns `string[]`
- [x] `ssh:listTeams` handler returns `string[]`
- [x] `ssh:filterTargets` handler accepts `SshTargetFilterCriteria`
- [x] `ssh:getAllConnectionStates` handler returns all states
- [x] TypeScript compile: no errors

**Status: ✅ DONE** — Handlers were added in TASK-005 session. `ssh:listTargetsByProject`, `ssh:listProjects`, `ssh:listTeams`, `ssh:filterTargets`, `ssh:getAllConnectionStates` all registered in `src/main/ipc/ssh.ts`.

---

# TASK-007: Thêm `SshTargetGroup` type và `groupSshTargetsByProject()` helper

**Source:** SOL-002  
**Phase:** 1 | **Effort:** XS (<30 min)  
**Depends on:** TASK-001

---

## Objective

Thêm shared type và helper function vào `src/shared/ssh-types.ts` để renderer có thể group targets locally (không cần IPC round-trip).

---

## File to modify

**`src/shared/ssh-types.ts`**

---

## Implementation

```typescript
// ── Fleet Grouping (NEW) ─────────────────────────────────────

/** A group of SSH targets belonging to the same project */
export type SshTargetGroup = {
  key: string           // project name or 'unassigned'
  label: string         // Display label (same as key, capitalized)
  targets: SshTarget[]
  isUnassigned: boolean
}

/**
 * Group a flat list of SshTargets by project.
 * Named projects appear first (sorted), 'Unassigned' last.
 * Safe to run in renderer process (pure function, no IPC).
 */
export function groupSshTargetsByProject(targets: SshTarget[]): SshTargetGroup[] {
  const map = new Map<string, SshTarget[]>()

  for (const target of targets) {
    const key = target.project ?? '__unassigned__'
    const group = map.get(key) ?? []
    group.push(target)
    map.set(key, group)
  }

  const groups: SshTargetGroup[] = []

  // Named projects first, sorted alphabetically
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
  if (unassigned && unassigned.length > 0) {
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

## Done criteria

- [x] `SshTargetGroup` type exported from `ssh-types.ts`
- [x] `groupSshTargetsByProject()` function exported
- [x] Named projects sorted alphabetically, Unassigned last
- [x] TypeScript compile: no errors

**Status: ✅ DONE** — `SshTargetGroup` type and `groupSshTargetsByProject()` added to `src/shared/ssh-types.ts`. Pure function, safe in renderer.

---

# TASK-008: Extend `buildRuntimeSyncWindowGraph()` với fleet metadata

**Source:** SOL-002  
**Phase:** 1 | **Effort:** XS (<30 min)  
**Depends on:** TASK-005

---

## Objective

Thêm `sshProjects` và `sshTeams` vào runtime sync graph để renderer nhận được fleet metadata khi sync.

---

## File to modify

**`src/main/runtime/orca-runtime.ts`** — tìm function `buildRuntimeSyncWindowGraph()` hoặc tương tự.

---

## Implementation

Trong function build sync graph, thêm:

```typescript
    // NEW: Fleet metadata
    sshProjects: sshConnectionStore.listProjects(),
    sshTeams: sshConnectionStore.listTeams(),
```

Và update type của `RuntimeSyncWindowGraph` (hoặc `RuntimeWindowGraph`) trong shared types:

```typescript
// src/shared/runtime-types.ts (hoặc nơi RuntimeSyncWindowGraph được define)
type RuntimeSyncWindowGraph = {
  // ... existing fields ...
  sshProjects?: string[]   // distinct project names
  sshTeams?: string[]      // distinct team names
}
```

---

## Done criteria

- [x] Sync graph includes `sshProjects: string[]`
- [x] Sync graph includes `sshTeams: string[]`
- [x] TypeScript compile: no errors

**Status: ✅ DONE** — Fleet metadata exposed via 2 mechanisms:
1. `listRegisteredSshProjects()` / `listRegisteredSshTeams()` exports added to `src/main/ipc/ssh.ts`
2. `ssh.listProjects` + `ssh.listTeams` RPC methods added to `src/main/runtime/rpc/methods/ssh.ts` (for web/mobile clients)

Note: `RuntimeSyncWindowGraph` not modified — the graph flows renderer→main, not main→renderer. Fleet metadata available via direct RPC calls instead.
