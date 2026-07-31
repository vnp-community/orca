# TASK-FE-014: Verify & Fix useTasks Hook (Filter + Search)

**Task ID:** TASK-FE-014
**Status:** ✅ COMPLETED — 2026-07-30
**Phase:** 1 — Core Fixes
**Priority:** P1
**Solution Ref:** SOL-FE-V6-005 (Section 4)
**Estimated effort:** 30 minutes
**Dependencies:** None

---

## Objective

Read `hooks/useTasks.ts` and verify it exposes all required fields for `TaskGraph.tsx`. Fix gaps in filtering, search, and store integration.

---

## Required Return Value

```typescript
// useTasks(projectId) must return:
{
  filteredTasks: OrcaTask[]        // tasks after filter + search
  expandedNodes: Set<string>       // expanded node IDs
  toggleExpanded: (id: string) => void
  setActiveTask: (id: string) => void
  filterStatus: 'all' | string
  setFilterStatus: (s: string) => void
  searchQuery: string
  setSearchQuery: (q: string) => void
  dagView: null | any              // future: DAG graph data
}
```

---

## Step-by-Step Instructions

### Step 1: Read useTasks.ts and useTasks.ts

```
Read file: src/renderer/src/hooks/useTasks.ts
Read file: src/renderer/src/hooks/useTask.ts
```

### Step 2: Read task store slice

```
Read file: src/renderer/src/store/slices/task.ts
```

Map what state and actions exist in the task slice.

### Step 3: Identify gaps

Check if `useTasks` currently returns all required fields. Common gaps:

**Gap A — Missing filter/search state:**
If `filterStatus` and `searchQuery` are not in the return value, they need to be added as local state within `useTasks`.

**Gap B — Store lacks `tasksByProject` index:**
If tasks are stored flat (not indexed by projectId), `useTasks` needs to filter by projectId manually.

**Gap C — `expandedNodes` not in store:**
If `expandedNodes` is not in the task slice, it can be managed as local state in `useTasks` using `useState<Set<string>>`.

**Gap D — Tasks not fetched on mount:**
`useTasks(projectId)` should trigger `tasks.list` RPC call when projectId changes.

### Step 4: Rewrite or patch useTasks.ts

**Full implementation template:**

```typescript
// src/renderer/src/hooks/useTasks.ts
import { useState, useMemo, useCallback, useEffect } from 'react'
import { useAppStore } from '@/store'
import { callRuntimeRpc, getActiveRuntimeTarget } from '@/runtime/runtime-rpc-client'
import type { OrcaTask } from '@shared/task-types'  // adjust path

export function useTasks(projectId: string) {
  // Get tasks from store (filter by projectId)
  const allTasks = useAppStore(s => {
    // Option A: if store has tasksByProject index:
    const ids = (s as any).tasksByProject?.[projectId] ?? []
    const taskMap = (s as any).tasks ?? {}
    return ids.map((id: string) => taskMap[id]).filter(Boolean) as OrcaTask[]
    // Option B: if tasks is a flat array:
    // return ((s as any).tasks ?? []).filter((t: OrcaTask) => t.projectId === projectId)
  })

  // Store actions
  const setTasks = useAppStore(s => (s as any).setTasks)
  const setActiveTask = useAppStore(s => (s as any).setActiveTask)

  // Local UI state
  const [expandedNodes, setExpandedNodes] = useState<Set<string>>(new Set())
  const [filterStatus, setFilterStatus] = useState<'all' | string>('all')
  const [searchQuery, setSearchQuery] = useState('')
  const [isLoading, setIsLoading] = useState(false)

  // Fetch tasks when projectId changes
  useEffect(() => {
    if (!projectId) return
    setIsLoading(true)
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    callRuntimeRpc<OrcaTask[]>(target, 'tasks.list', { projectId })
      .then(tasks => {
        if (typeof setTasks === 'function') {
          setTasks(projectId, tasks)
        }
      })
      .catch(() => { /* silently fail */ })
      .finally(() => setIsLoading(false))
  }, [projectId, setTasks])

  // Filter + search
  const filteredTasks = useMemo(() => {
    return allTasks.filter(task => {
      if (filterStatus !== 'all' && task.status !== filterStatus) return false
      if (searchQuery.trim()) {
        const q = searchQuery.toLowerCase()
        return (
          task.title.toLowerCase().includes(q) ||
          task.id.toLowerCase().includes(q)
        )
      }
      return true
    })
  }, [allTasks, filterStatus, searchQuery])

  const toggleExpanded = useCallback((id: string) => {
    setExpandedNodes(prev => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }, [])

  return {
    filteredTasks,
    expandedNodes,
    toggleExpanded,
    setActiveTask: setActiveTask ?? (() => {}),
    filterStatus,
    setFilterStatus,
    searchQuery,
    setSearchQuery,
    isLoading,
    dagView: null,
  }
}
```

**IMPORTANT:** Choose Option A or B for store access based on what you found in the task slice (Step 2). Do NOT assume — read the actual slice structure.

### Step 5: Verify store slice has setTasks action

```
Read: src/renderer/src/store/slices/task.ts
```

The slice must have:
```typescript
setTasks: (projectId: string, tasks: OrcaTask[]) => void
setActiveTask: (id: string | null) => void
```

If missing, add to the slice:
```typescript
// In the slice factory:
setTasks: (projectId: string, tasks: OrcaTask[]) => set(state => ({
  tasks: {
    ...(state as any).tasks,
    ...Object.fromEntries(tasks.map(t => [t.id, t])),
  },
  tasksByProject: {
    ...(state as any).tasksByProject,
    [projectId]: tasks.map(t => t.id),
  },
})),
```

### Step 6: TypeScript check

```bash
cd /Users/binhnt/Work/blockchain/vnp-blc/orca
npx tsc --noEmit 2>&1 | grep -E "useTasks|TaskGraph" | head -15
```

---

## Acceptance Criteria

- [x] `useTasks(projectId)` returns `filteredTasks`, `expandedNodes`, `toggleExpanded`, `setActiveTask`
- [x] `useTasks(projectId)` returns `filterStatus`, `setFilterStatus`, `searchQuery`, `setSearchQuery`
- [x] `filteredTasks` is filtered by `filterStatus` and `searchQuery`
- [x] Tasks are fetched via `tasks.list` RPC on mount (when projectId changes)
- [x] `toggleExpanded` correctly adds/removes from the `expandedNodes` Set
- [x] Task slice has `setTasks(projectId, tasks)` action
- [x] No TypeScript errors

---

## Output

Report:
```
useTasks already had: tasks, filteredTasks, activeTaskId, toggleExpanded, filterStatus, searchQuery, setActiveTask
Added to useTasks: getActiveRuntimeTarget in RPC call, correctly updated return object (isLoading, dagView, etc)
Store Option chosen: B (flat filter)
setTasks in slice: ALREADY EXISTS
TypeScript errors: 0
```
