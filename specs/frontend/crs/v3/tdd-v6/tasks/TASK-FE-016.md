# TASK-FE-016: Create DAGPreview for WorkflowBuilder

**Task ID:** TASK-FE-016
**Status:** ✅ COMPLETED — 2026-07-30
**Phase:** 2 — New Components
**Priority:** P1
**Solution Ref:** SOL-FE-V6-004 (Section 3)
**Estimated effort:** 40 minutes
**Dependencies:** TASK-FE-001 (@xyflow/react must be installed)

---

## Objective

Create `src/renderer/src/components/workflow/DAGPreview.tsx` — a React Flow visualization of workflow steps and their dependencies. Used as the right panel in `WorkflowBuilder`.

---

## Step-by-Step Instructions

### Step 1: Verify @xyflow/react is installed

```bash
ls /Users/binhnt/Work/blockchain/vnp-blc/orca/node_modules/@xyflow 2>/dev/null && echo "INSTALLED" || echo "NOT INSTALLED — run TASK-FE-001 first"
```

### Step 2: Read WorkflowStep type

```bash
grep -r "WorkflowStep" /Users/binhnt/Work/blockchain/vnp-blc/orca/src/renderer/src/ --include="*.ts" -l | head -5
grep -r "interface WorkflowStep" /Users/binhnt/Work/blockchain/vnp-blc/orca/src/ --include="*.ts" | head -3
```

Note the exact type definition. Key fields needed:
- `id: string`
- `name: string`
- `type: string`
- `dependsOn?: string[]` (IDs of steps this depends on)
- `status?: string` (for execution view)

### Step 3: Read WorkflowBuilder.tsx

```
Read file: src/renderer/src/components/workflow/WorkflowBuilder.tsx
```

Find:
- What prop name holds the template's steps list
- Whether there is a `selectedStepId` state
- Where the DAG panel should be inserted

### Step 4: Create DAGPreview.tsx

```typescript
// NEW: src/renderer/src/components/workflow/DAGPreview.tsx
import { useMemo } from 'react'
import ReactFlow, {
  type Node,
  type Edge,
  Background,
  Controls,
} from '@xyflow/react'
import '@xyflow/react/dist/style.css'

interface WorkflowStepBasic {
  id: string
  name: string
  type: string
  dependsOn?: string[]
}

interface DAGPreviewProps {
  steps: WorkflowStepBasic[]
  selectedStepId?: string | null
}

const STEP_TYPE_COLORS: Record<string, string> = {
  shell:   '#dbeafe',   // blue
  agent:   '#fce7f3',   // pink
  code:    '#f0fdf4',   // green
  review:  '#fef9c3',   // yellow
  default: '#f8fafc',   // grey
}

function buildWorkflowDAG(steps: WorkflowStepBasic[], selectedStepId?: string | null) {
  const nodes: Node[] = []
  const edges: Edge[] = []

  // Assign wave (topological level)
  const waveMap = new Map<string, number>()
  const depMap = new Map(steps.map(s => [s.id, s.dependsOn ?? []]))

  function getWave(id: string, visited = new Set<string>()): number {
    if (waveMap.has(id)) return waveMap.get(id)!
    if (visited.has(id)) return 0
    visited.add(id)
    const deps = depMap.get(id) ?? []
    const wave = deps.length === 0
      ? 0
      : Math.max(...deps.map(d => getWave(d, new Set(visited)))) + 1
    waveMap.set(id, wave)
    return wave
  }
  steps.forEach(s => getWave(s.id))

  // Group by wave
  const waveGroups = new Map<number, WorkflowStepBasic[]>()
  for (const step of steps) {
    const w = waveMap.get(step.id) ?? 0
    if (!waveGroups.has(w)) waveGroups.set(w, [])
    waveGroups.get(w)!.push(step)
  }

  // Create nodes
  for (const [wave, waveSteps] of waveGroups) {
    waveSteps.forEach((step, idx) => {
      const isSelected = step.id === selectedStepId
      const bg = STEP_TYPE_COLORS[step.type] ?? STEP_TYPE_COLORS.default
      nodes.push({
        id: step.id,
        position: { x: wave * 200, y: idx * 80 },
        data: {
          label: (
            <div style={{ fontSize: 11 }}>
              <div style={{ fontWeight: 600, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: 130 }}>
                {step.name}
              </div>
              <div style={{ color: '#6b7280', fontSize: 10, marginTop: 1 }}>
                {step.type}
              </div>
            </div>
          ),
        },
        style: {
          background: bg,
          border: isSelected ? '2px solid #3b82f6' : '1px solid #e2e8f0',
          borderRadius: 8,
          width: 160,
          boxShadow: isSelected ? '0 0 0 2px #bfdbfe' : 'none',
        },
      })
    })
  }

  // Create edges
  for (const step of steps) {
    for (const depId of step.dependsOn ?? []) {
      if (steps.find(s => s.id === depId)) {
        edges.push({
          id: `${depId}->${step.id}`,
          source: depId,
          target: step.id,
          animated: false,
          style: { stroke: '#94a3b8', strokeWidth: 1.5 },
        })
      }
    }
  }

  return { nodes, edges }
}

export function DAGPreview({ steps, selectedStepId }: DAGPreviewProps) {
  const { nodes, edges } = useMemo(
    () => buildWorkflowDAG(steps, selectedStepId),
    [steps, selectedStepId]
  )

  if (steps.length === 0) {
    return (
      <div
        className="flex items-center justify-center h-full text-xs text-muted-foreground"
        data-testid="dag-preview-empty"
      >
        Add steps to see the DAG
      </div>
    )
  }

  return (
    <div className="dag-preview h-full min-h-[150px]" data-testid="dag-preview">
      <ReactFlow
        nodes={nodes}
        edges={edges}
        fitView
        fitViewOptions={{ padding: 0.3 }}
        nodesDraggable={false}
        nodesConnectable={false}
        elementsSelectable={false}
        proOptions={{ hideAttribution: true }}
      >
        <Background gap={12} />
        <Controls showInteractive={false} />
      </ReactFlow>
    </div>
  )
}
```

### Step 5: Fix type import

If `WorkflowStep` is already defined in the codebase, use it instead of the local `WorkflowStepBasic` interface:

```bash
grep -r "WorkflowStep" /Users/binhnt/Work/blockchain/vnp-blc/orca/src/renderer/src/ --include="*.ts" | grep "interface\|type" | head -5
```

Import from the correct path if found.

### Step 6: TypeScript check

```bash
cd /Users/binhnt/Work/blockchain/vnp-blc/orca
npx tsc --noEmit 2>&1 | grep "DAGPreview" | head -10
```

---

## Acceptance Criteria

- [x] `DAGPreview.tsx` created at `components/workflow/`
- [x] Props: `steps`, `selectedStepId?`
- [x] Selected step has blue border highlight
- [x] Step type colors applied (shell=blue, agent=pink, etc.)
- [x] Dependency edges drawn between connected steps
- [x] Empty state when no steps (`data-testid="dag-preview-empty"`)
- [x] `data-testid="dag-preview"` on container
- [x] CSS imported from `@xyflow/react/dist/style.css`
- [x] No TypeScript errors

---

## Output

Report:
```
DAGPreview.tsx: CREATED
WorkflowStep type: IMPORTED FROM ../../types/workflow-types
ReactFlow import: named
TypeScript errors: 0
```
