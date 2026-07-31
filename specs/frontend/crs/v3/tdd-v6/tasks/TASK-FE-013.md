# TASK-FE-013: Implement TaskDAGView with React Flow

**Task ID:** TASK-FE-013
**Status:** ✅ COMPLETED — 2026-07-30
**Phase:** 2 — New Components
**Priority:** P1
**Solution Ref:** SOL-FE-V6-005 (Section 2)
**Estimated effort:** 45 minutes
**Dependencies:** TASK-FE-001 (@xyflow/react must be installed)

---

## Objective

Replace the stub `TaskDAGView.tsx` (615 bytes) with a full React Flow implementation. The DAG view shows tasks as nodes positioned by dependency level (wave layout), with edges connecting dependent tasks.

---

## Context

**File to replace:** `src/renderer/src/components/task/TaskDAGView.tsx` (current stub, 615 bytes)

**TDD-FE-15 requirement:** Show tasks in a directed acyclic graph using React Flow. Each node shows task title and type. Nodes are colored based on status. Clicking a node triggers `onSelect(taskId)`.

---

## Step-by-Step Instructions

### Step 1: Verify @xyflow/react is installed

```bash
ls /Users/binhnt/Work/blockchain/vnp-blc/orca/node_modules/@xyflow 2>/dev/null && echo "INSTALLED" || echo "NOT INSTALLED — run TASK-FE-001 first"
```

### Step 2: Read current TaskDAGView.tsx

```
Read file: src/renderer/src/components/task/TaskDAGView.tsx
```

### Step 3: Read OrcaTask type

```
Read file: src/renderer/src/types/task-types.ts (or wherever OrcaTask is defined)
```

OR:
```bash
grep -r "OrcaTask" /Users/binhnt/Work/blockchain/vnp-blc/orca/src/ --include="*.ts" -l | head -5
```

Check fields: `id`, `title`, `type`, `status`, `parentId`, `dependsOn`.

### Step 4: Replace TaskDAGView.tsx

```typescript
// REPLACE: src/renderer/src/components/task/TaskDAGView.tsx
import { useMemo, useCallback } from 'react'
import ReactFlow, {
  type Node,
  type Edge,
  Background,
  Controls,
  MiniMap,
  type OnNodeClick,
} from '@xyflow/react'
import '@xyflow/react/dist/style.css'
import type { OrcaTask } from '@shared/task-types'  // adjust import path if needed

interface TaskDAGViewProps {
  tasks: OrcaTask[]
  onSelect: (taskId: string) => void
}

// Color coding by status
const STATUS_COLORS: Record<string, { bg: string; border: string }> = {
  done:        { bg: '#f0fdf4', border: '#16a34a' },
  in_progress: { bg: '#eff6ff', border: '#2563eb' },
  blocked:     { bg: '#fef2f2', border: '#dc2626' },
  review:      { bg: '#faf5ff', border: '#9333ea' },
  todo:        { bg: '#f8fafc', border: '#94a3b8' },
  backlog:     { bg: '#f8fafc', border: '#cbd5e1' },
  cancelled:   { bg: '#f1f5f9', border: '#64748b' },
}

function buildDAGLayout(tasks: OrcaTask[]): { nodes: Node[]; edges: Edge[] } {
  if (tasks.length === 0) return { nodes: [], edges: [] }

  // Build dependency map: taskId -> list of taskIds this depends on
  const dependsOnMap = new Map<string, string[]>()
  for (const task of tasks) {
    dependsOnMap.set(task.id, (task as any).dependsOn ?? [])
  }

  // Topological wave assignment
  const waveMap = new Map<string, number>()
  function getWave(id: string, visited = new Set<string>()): number {
    if (waveMap.has(id)) return waveMap.get(id)!
    if (visited.has(id)) return 0  // cycle guard
    visited.add(id)
    const deps = dependsOnMap.get(id) ?? []
    const wave = deps.length === 0
      ? 0
      : Math.max(...deps.map(d => getWave(d, new Set(visited)))) + 1
    waveMap.set(id, wave)
    return wave
  }
  tasks.forEach(t => getWave(t.id))

  // Group tasks by wave
  const waveGroups = new Map<number, OrcaTask[]>()
  for (const task of tasks) {
    const wave = waveMap.get(task.id) ?? 0
    if (!waveGroups.has(wave)) waveGroups.set(wave, [])
    waveGroups.get(wave)!.push(task)
  }

  // Position and create nodes
  const nodes: Node[] = []
  const HORIZONTAL_GAP = 220
  const VERTICAL_GAP = 90

  for (const [wave, waveTasks] of waveGroups) {
    waveTasks.forEach((task, idx) => {
      const colors = STATUS_COLORS[task.status] ?? STATUS_COLORS.todo
      nodes.push({
        id: task.id,
        position: { x: wave * HORIZONTAL_GAP, y: idx * VERTICAL_GAP },
        data: {
          label: (
            <div style={{ fontSize: 11, padding: '2px 4px' }}>
              <div style={{ fontWeight: 600, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: 140 }}>
                {task.title}
              </div>
              <div style={{ color: '#6b7280', marginTop: 2 }}>
                [{task.type}]
              </div>
            </div>
          ),
        },
        style: {
          background: colors.bg,
          border: `2px solid ${colors.border}`,
          borderRadius: 8,
          width: 170,
          minHeight: 50,
        },
      })
    })
  }

  // Create dependency edges
  const edges: Edge[] = []
  for (const task of tasks) {
    const deps = (task as any).dependsOn ?? []
    for (const depId of deps) {
      if (tasks.find(t => t.id === depId)) {
        edges.push({
          id: `${depId}->${task.id}`,
          source: depId,
          target: task.id,
          animated: task.status === 'in_progress',
          style: { stroke: '#94a3b8', strokeWidth: 1.5 },
        })
      }
    }
  }

  return { nodes, edges }
}

export function TaskDAGView({ tasks, onSelect }: TaskDAGViewProps) {
  const { nodes, edges } = useMemo(() => buildDAGLayout(tasks), [tasks])

  const onNodeClick: OnNodeClick = useCallback((_event, node) => {
    onSelect(node.id)
  }, [onSelect])

  if (tasks.length === 0) {
    return (
      <div
        className="flex items-center justify-center h-full text-sm text-muted-foreground"
        data-testid="task-dag-empty"
      >
        No tasks to display
      </div>
    )
  }

  return (
    <div className="task-dag-view h-full min-h-[300px]" data-testid="task-dag-view">
      <ReactFlow
        nodes={nodes}
        edges={edges}
        onNodeClick={onNodeClick}
        fitView
        fitViewOptions={{ padding: 0.2 }}
        nodesDraggable={false}
        nodesConnectable={false}
        elementsSelectable={true}
        proOptions={{ hideAttribution: true }}
      >
        <Background gap={16} />
        <Controls showInteractive={false} />
        <MiniMap nodeStrokeWidth={2} />
      </ReactFlow>
    </div>
  )
}

export default TaskDAGView
```

### Step 5: Fix OrcaTask import path

The import `from '@shared/task-types'` may not be correct. Check:

```bash
grep -r "OrcaTask" /Users/binhnt/Work/blockchain/vnp-blc/orca/src/renderer/src/ --include="*.ts" --include="*.tsx" | grep "import" | head -5
```

Use the same import path found in other task components.

### Step 6: Fix ReactFlow imports

ReactFlow v12 (`@xyflow/react`) has different exports than v11. Check:

```bash
node -e "const m = require('@xyflow/react'); console.log(Object.keys(m).slice(0, 20))"
```

If `ReactFlow` is the default export, use it. If it's a named export, adjust.

Common v12 pattern:
```typescript
import { ReactFlow, Background, Controls, MiniMap } from '@xyflow/react'
import '@xyflow/react/dist/style.css'
```

### Step 7: TypeScript check

```bash
cd /Users/binhnt/Work/blockchain/vnp-blc/orca
npx tsc --noEmit 2>&1 | grep -E "TaskDAGView|ReactFlow|@xyflow" | head -15
```

---

## Acceptance Criteria

- [x] `TaskDAGView.tsx` uses `ReactFlow` / `@xyflow/react`
- [x] Props: `tasks: OrcaTask[]`, `onSelect: (id: string) => void`
- [x] Nodes positioned by dependency wave (tasks with no deps in wave 0)
- [x] Node color reflects task status (done=green, blocked=red, etc.)
- [x] Clicking a node calls `onSelect(task.id)`
- [x] Empty state shows message when no tasks
- [x] `data-testid="task-dag-view"` on container
- [x] `data-testid="task-dag-empty"` on empty state
- [x] CSS file imported (`@xyflow/react/dist/style.css`)
- [x] No TypeScript errors

---

## Output

Report:
```
TaskDAGView.tsx replaced: YES
@xyflow/react version: 12.x
ReactFlow import: named (ReactFlow, Background, Controls, MiniMap)
OrcaTask import path: ../../types/task-types
dependsOn field: FOUND in OrcaTask
TypeScript errors: 0
```
