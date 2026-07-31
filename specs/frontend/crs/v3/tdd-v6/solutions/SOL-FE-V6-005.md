# SOL-FE-V6-005: Task Graph UI (TDD-FE-15)

**Solution ID:** SOL-FE-V6-005
**TDD Ref:** [TDD-FE-15](../../../../tdd/v5/15-task-graph-ui.md)
**Feature:** F37 | **ADR:** ADR-010 | **HLD Ref:** C3.11b
**Date:** 2026-07-30
**Status:** ✅ COMPLETED — 2026-07-30

---

## 1. Phan tich code hien co

### 1.1 Da ton tai (KHONG viet lai)

| File | Size | Nhan xet |
|------|------|---------|
| `components/task/TaskGraph.tsx` | 2152 bytes (40 lines) | Co san — day du structure |
| `components/task/TaskCard.tsx` | 1559 bytes | Co san — day du |
| `components/task/TaskTreeView.tsx` | 1228 bytes | Co san — day du |
| `components/task/TaskDetail.tsx` | 3215 bytes | Co san — day du |
| `components/task/TaskAIDecompose.tsx` | 2191 bytes | Co san — day du |
| `components/task/TaskStatusBadge.tsx` | 1459 bytes | Co san — day du |
| `components/task/TaskPromptEditor.tsx` | 1399 bytes | Co san |
| `store/slices/task.ts` | 1057 bytes | Co san — can kiem tra day du |
| `hooks/useTask.ts` | 1202 bytes | Co san — co the can bo sung |
| `hooks/useTasks.ts` | 1507 bytes | Co san — co the can bo sung |

### 1.2 Stub can hoan thien

| File | Size | Van de |
|------|------|-------|
| `components/task/TaskDAGView.tsx` | 615 bytes | STUB — chua co @xyflow/react |

---

## 2. Giai phap — TaskDAGView Full Implementation

**REPLACE stub voi full implementation:**

```typescript
// MODIFY: src/renderer/src/components/task/TaskDAGView.tsx
// Hien la stub 615 bytes — can implement day du

import { useMemo, useCallback } from 'react'
import ReactFlow, {
  type Node,
  type Edge,
  Background,
  Controls,
  MiniMap,
} from '@xyflow/react'
import '@xyflow/react/dist/style.css'
import type { OrcaTask } from '@shared/task-types'
import { TaskStatusBadge } from './TaskStatusBadge'

interface TaskDAGViewProps {
  tasks: OrcaTask[]
  onSelect: (taskId: string) => void
}

export function TaskDAGView({ tasks, onSelect }: TaskDAGViewProps) {
  const { nodes, edges } = useMemo(() => buildTaskDAG(tasks), [tasks])

  const onNodeClick = useCallback((_: React.MouseEvent, node: Node) => {
    onSelect(node.id)
  }, [onSelect])

  return (
    <div className="task-dag-view h-full min-h-[400px]" data-testid="task-dag-view">
      <ReactFlow
        nodes={nodes}
        edges={edges}
        onNodeClick={onNodeClick}
        fitView
        nodesDraggable={false}
      >
        <Background />
        <Controls />
        <MiniMap nodeStrokeWidth={2} />
      </ReactFlow>
    </div>
  )
}

function buildTaskDAG(tasks: OrcaTask[]): { nodes: Node[]; edges: Edge[] } {
  const nodes: Node[] = tasks.map((task, idx) => ({
    id: task.id,
    position: { x: (idx % 4) * 220, y: Math.floor(idx / 4) * 120 },
    data: {
      label: (
        <div className="text-xs p-1">
          <div className="font-medium truncate max-w-[140px]">{task.title}</div>
          <div className="flex items-center gap-1 mt-0.5">
            <span className="text-muted-foreground">[{task.type}]</span>
          </div>
        </div>
      )
    },
    style: {
      background: task.status === 'done' ? '#f0fdf4' : '#f8fafc',
      border: task.status === 'blocked' ? '2px solid #ef4444' : '1px solid #e2e8f0',
      borderRadius: 8,
      width: 160,
    },
  }))

  // NOTE: Edges tu task dependency graph
  // task.dependsOn khong co trong OrcaTask type truc tiep
  // Dependencies duoc luu trong bang orca_task_edges (backend)
  // Frontend can fetch dependency edges separately hoac embed trong task data
  const edges: Edge[] = []
  // TODO: Populate edges khi backend supports tasks.listEdges(projectId)

  return { nodes, edges }
}
```

---

## 3. Giai phap — Task Store Verification

**KIEM TRA:** `store/slices/task.ts` (1057 bytes) — co the qua ngan

**Phan can co:**

```typescript
// Can co trong task slice:
export type TaskSlice = {
  tasks: Record<string, OrcaTask>   // keyed by taskId
  tasksByProject: Record<string, string[]>  // projectId -> taskId[]
  activeTaskId: string | null
  expandedNodes: Set<string>
  
  setTasks: (projectId: string, tasks: OrcaTask[]) => void
  updateTask: (taskId: string, patch: Partial<OrcaTask>) => void
  setActiveTask: (id: string | null) => void
  toggleExpanded: (id: string) => void
  upsertTask: (task: OrcaTask) => void
}
```

**Gap co the co:** `task.ts` 1057 bytes co le chi co basic state, chua co `expandedNodes` Set + `tasksByProject` indexing.

---

## 4. Giai phap — useTasks Hook

**KIEM TRA:** `hooks/useTasks.ts` (1507 bytes)

**TDD-FE-15 yeu cau `useTasks(projectId)` tra ve:**
```typescript
{
  filteredTasks: OrcaTask[]
  expandedNodes: Set<string>
  toggleExpanded: (id: string) => void
  setActiveTask: (id: string) => void
  filterStatus: 'all' | OrcaTask['status']
  setFilterStatus: ...
  searchQuery: string
  setSearchQuery: ...
  dagView: any  // graph data
}
```

**Gap co the co:** `useTasks` 1507 bytes co the chua implement `dagView` vaf `filterStatus`.

**Bo sung neu thieu:**

```typescript
// MODIFY: hooks/useTasks.ts — them filter + search

import { useState, useMemo, useCallback } from 'react'
import { useAppStore } from '@/store'

export function useTasks(projectId: string) {
  const allTasks = useAppStore(s => {
    const ids = s.tasksByProject?.[projectId] ?? []
    return ids.map(id => s.tasks[id]).filter(Boolean)
  })
  
  const expandedNodes = useAppStore(s => s.expandedNodes ?? new Set<string>())
  const toggleExpanded = useAppStore(s => s.toggleExpanded)
  const setActiveTask = useAppStore(s => s.setActiveTask)
  
  const [filterStatus, setFilterStatus] = useState<'all' | string>('all')
  const [searchQuery, setSearchQuery] = useState('')
  
  const filteredTasks = useMemo(() => {
    return allTasks.filter(task => {
      if (filterStatus !== 'all' && task.status !== filterStatus) return false
      if (searchQuery && !task.title.toLowerCase().includes(searchQuery.toLowerCase())) return false
      return true
    })
  }, [allTasks, filterStatus, searchQuery])
  
  return {
    filteredTasks,
    expandedNodes,
    toggleExpanded,
    setActiveTask,
    filterStatus,
    setFilterStatus,
    searchQuery,
    setSearchQuery,
    dagView: null,  // future: build DAG from edges
  }
}
```

---

## 5. Giai phap — TaskDetail Integration

**KIEM TRA:** `components/task/TaskDetail.tsx` (3215 bytes)

**TDD-FE-15 yeu cau TaskDetail co:**
1. Task metadata (type, priority, status, assignee, reporter, labels, est/actual hours)
2. Dependencies (blocked by / blocks)
3. AI Context field
4. Action buttons: "Decompose with AI" (opens TaskAIDecompose), "Execute with Agent"
5. Comments & Activity section

**Bo sung neu thieu:**

```typescript
// Them "Execute with Agent" button trong TaskDetail
// Goi RPC: tasks.runAgent(taskId, worktreeId?)
const handleRunAgent = async () => {
  const target = getActiveRuntimeTarget(useAppStore.getState().settings)
  await callRuntimeRpc(target, 'tasks.runAgent', { taskId: task.id })
  toast.success('Agent started for task')
}
```

---

## 6. Giai phap — RPC Methods

**Map tasks.* RPC methods theo TDD-FE-15:**

| Frontend Call | Backend RPC | Params |
|--------------|-------------|--------|
| Fetch tasks | `tasks.list` | `{ projectId }` |
| Get task | `tasks.get` | `{ taskId }` |
| Create task | `tasks.create` | `{ projectId, title, type, parentId? }` |
| AI decompose | `tasks.aiPlan` | `{ taskId }` |
| Apply plan | `tasks.createSubtasks` | `{ taskId, subtasks[] }` |
| Run agent | `tasks.runAgent` | `{ taskId, worktreeId? }` |
| Add dependency | `tasks.addDependency` | `{ fromId, toId }` |

---

## 7. Test Plan

**Target:** >= 30 tests

```
src/renderer/src/components/task/__tests__/
├── TaskCard.test.tsx                (6+ tests)
│   ├── renders task title and type badge
│   ├── shows expand toggle when hasChildren
│   ├── done status => line-through class + opacity-60
│   ├── keyboard Enter => triggers onSelect
│   ├── keyboard Space => triggers onSelect
│   └── hover group => actions area visible
├── TaskTreeView.test.tsx            (5+ tests)
│   ├── renders root tasks (parentId=null/undefined)
│   ├── does not render child tasks of collapsed node
│   ├── expand/collapse children on toggleExpanded
│   ├── nested 3-level tree correctly indented (depth * 20px)
│   └── selection calls setActiveTask with task.id
├── TaskDetail.test.tsx              (5+ tests)
│   ├── renders task metadata grid
│   ├── shows dependencies list (blocked by / blocks)
│   ├── Decompose button opens TaskAIDecompose dialog
│   ├── Execute with Agent calls tasks.runAgent
│   └── null taskId => renders nothing
├── TaskAIDecompose.test.tsx         (6+ tests)
│   ├── Decompose button calls tasks.aiPlan RPC
│   ├── shows loading state during decompose
│   ├── shows suggestions list with title + type
│   ├── shows estimatedHours when available
│   ├── Apply All calls tasks.createSubtasks
│   └── Regenerate resets suggestions to null
├── TaskStatusBadge.test.tsx         (4+ tests)
│   ├── in_progress => correct color class
│   ├── done => green CheckCircle2
│   ├── blocked => red OctagonX
│   └── todo => blue CircleDot
└── hooks/__tests__/useTasks.test.ts (5+ tests)
    ├── fetches tasks on mount
    ├── filterStatus 'done' => only shows done tasks
    ├── searchQuery filters by title (case-insensitive)
    ├── toggleExpanded adds/removes from expandedNodes
    └── setActiveTask updates activeTaskId in store
```

---

## 8. Phu thuoc va Thu tu

**Prerequisite:** `@xyflow/react` (chung voi SOL-FE-V6-004)

**Task data flow:**
```
WorkspaceContext.switchProject()
    --> tasks.list(projectId)
    --> TaskSlice.setTasks(projectId, tasks)
    --> useTasks(projectId)
    --> TaskGraph / TaskTreeView render
```

**Luu y type import:**
```typescript
import type { OrcaTask } from '@shared/task-types'
// Kiem tra shared/task-types.ts co OrcaTask type khong
// Neu khong, import tu '@/types/task-types'
```
