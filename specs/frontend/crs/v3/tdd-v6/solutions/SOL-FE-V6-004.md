# SOL-FE-V6-004: Workflow Builder & Monitor (TDD-FE-14)

**Solution ID:** SOL-FE-V6-004
**TDD Ref:** [TDD-FE-14](../../../../tdd/v5/14-workflow-ui.md)
**Feature:** F36 | **ADR:** ADR-009 | **HLD Ref:** C3.11c
**Date:** 2026-07-30
**Status:** ✅ COMPLETED — 2026-07-30

---

## 1. Phan tich code hien co

### 1.1 Da ton tai (KHONG viet lai)

| File | Size | Nhan xet |
|------|------|---------|
| `components/workflow/WorkflowBuilder.tsx` | 3199 bytes | Co san — can kiem tra DAGPreview integration |
| `components/workflow/StepEditor.tsx` | 1986 bytes | Co san — day du |
| `components/workflow/StepList.tsx` | 2262 bytes | Co san — day du |
| `components/workflow/StepStatusBadge.tsx` | 1090 bytes | Co san — day du |
| `components/workflow/ExecutionMonitor.tsx` | 3295 bytes | Co san — can kiem tra streaming output |
| `store/slices/workflow.ts` | 2360 bytes | Co san — day du |
| `hooks/useWorkflow.ts` | 2484 bytes | Co san — day du |
| `hooks/useWorkflowExecution.ts` | 1514 bytes | Co san — day du |

### 1.2 Chua ton tai (CAN TAO MOI)

| File | TDD Ref | Do uu tien |
|------|---------|-----------|
| `components/workflow/DAGPreview.tsx` | Section 3 | HIGH |
| `components/workflow/__tests__/*.test.tsx` | Section 6 | HIGH |

---

## 2. Dependency Required: @xyflow/react

**TRUOC KHI implement DAGPreview, can install:**

```bash
# Kiem tra xem da co chua
ls node_modules/@xyflow/react 2>/dev/null && echo "INSTALLED" || echo "NOT INSTALLED"

# Neu chua co:
npm install @xyflow/react
# hoac
pnpm add @xyflow/react
```

**Tai sao can @xyflow/react:**
- `DAGPreview` trong WorkflowBuilder su dung React Flow de ve DAG nodes + edges
- `TaskDAGView` (TDD-FE-15) cung su dung cung thu vien nay
- Doi khi goi la `reactflow` (v11) hoac `@xyflow/react` (v12)

---

## 3. Giai phap — DAGPreview Component

**File moi:** `src/renderer/src/components/workflow/DAGPreview.tsx`

```typescript
// NEW: src/renderer/src/components/workflow/DAGPreview.tsx
// Su dung @xyflow/react de ve DAG preview cua workflow steps

import { useMemo } from 'react'
import ReactFlow, { type Node, type Edge, Background, Controls } from '@xyflow/react'
import '@xyflow/react/dist/style.css'
import type { WorkflowStep } from '@/types/workflow-types'

interface DAGPreviewProps {
  steps: WorkflowStep[]
  selectedStepId: string | null
}

export function DAGPreview({ steps, selectedStepId }: DAGPreviewProps) {
  const { nodes, edges } = useMemo(() => buildDAGLayout(steps, selectedStepId), [steps, selectedStepId])

  return (
    <div className="dag-preview h-full min-h-[200px]" data-testid="dag-preview">
      <ReactFlow
        nodes={nodes}
        edges={edges}
        fitView
        nodesDraggable={false}
        nodesConnectable={false}
        elementsSelectable={false}
      >
        <Background />
        <Controls showInteractive={false} />
      </ReactFlow>
    </div>
  )
}

function buildDAGLayout(
  steps: WorkflowStep[],
  selectedStepId: string | null
): { nodes: Node[]; edges: Edge[] } {
  const nodes: Node[] = []
  const edges: Edge[] = []
  const waveMap = new Map<string, number>()

  // Topological sort: assign wave numbers
  function assignWave(stepId: string): number {
    if (waveMap.has(stepId)) return waveMap.get(stepId)!
    const step = steps.find(s => s.id === stepId)
    if (!step) return 0
    const deps = step.dependsOn ?? []
    const wave = deps.length === 0
      ? 0
      : Math.max(...deps.map(d => assignWave(d))) + 1
    waveMap.set(stepId, wave)
    return wave
  }
  steps.forEach(s => assignWave(s.id))

  // Group by wave
  const waveGroups = new Map<number, string[]>()
  for (const [id, wave] of waveMap) {
    if (!waveGroups.has(wave)) waveGroups.set(wave, [])
    waveGroups.get(wave)!.push(id)
  }

  // Position nodes
  for (const [wave, ids] of waveGroups) {
    ids.forEach((id, idx) => {
      const step = steps.find(s => s.id === id)!
      const isSelected = id === selectedStepId
      nodes.push({
        id,
        position: { x: wave * 200, y: idx * 80 },
        data: { label: `${step.name}\n(${step.type})` },
        type: 'default',
        style: {
          background: isSelected ? '#dbeafe' : '#f8fafc',
          border: isSelected ? '2px solid #3b82f6' : '1px solid #e2e8f0',
          borderRadius: 8,
          fontSize: 11,
          padding: '6px 10px',
        },
      })
    })
  }

  // Create edges
  for (const step of steps) {
    for (const dep of step.dependsOn ?? []) {
      edges.push({
        id: `${dep}-${step.id}`,
        source: dep,
        target: step.id,
        animated: true,
        style: { stroke: '#94a3b8' },
      })
    }
  }

  return { nodes, edges }
}
```

---

## 4. Giai phap — WorkflowBuilder Integration

**MODIFY:** `src/renderer/src/components/workflow/WorkflowBuilder.tsx`

**Gap hien tai:** WorkflowBuilder co 3-panel layout nhung chua integrate DAGPreview.

**Bo sung DAGPreview vao WorkflowBuilder:**

```typescript
// Them import DAGPreview (sau khi tao file tren):
import { lazy, Suspense } from 'react'
const DAGPreview = lazy(() => import('./DAGPreview').then(m => ({ default: m.DAGPreview })))

// Trong JSX cua WorkflowBuilder, trong panel DAG:
{showDagPreview && (
  <div className="dag-panel border-l w-64 overflow-hidden">
    <div className="p-2 text-xs text-muted-foreground border-b">DAG Preview</div>
    <Suspense fallback={<div className="p-3 text-xs">Loading DAG...</div>}>
      <DAGPreview
        steps={localTemplate.steps ?? []}
        selectedStepId={selectedStepId}
      />
    </Suspense>
  </div>
)}
```

---

## 5. Giai phap — ExecutionMonitor Streaming

**MODIFY:** `src/renderer/src/components/workflow/ExecutionMonitor.tsx`

**Gap:** ExecutionMonitor co 3295 bytes nhung chua co streaming output display.

**TDD-FE-14 yeu cau:** Show streaming output cho running steps via `rpc.callStream`.

**Kiem tra `useWorkflowExecution.ts` co handle streaming khong:**

```typescript
// useWorkflowExecution.ts nen co:
const streamOutput = useCallback(async (stepId: string, executionId: string) => {
  // rpc.callStream('workflow.streamStepOutput', { stepId })
  // --> append lines den streamingOutput state
}, [])
```

---

## 6. Giai phap — Workflow Slice Verification

**Kiem tra `store/slices/workflow.ts` co day du:**

```typescript
// Can co trong workflow slice:
export type WorkflowSlice = {
  templates: WorkflowTemplate[]
  executions: WorkflowExecution[]
  streamingOutput: Record<string, string[]>  // executionId -> lines
  
  setTemplates: (t: WorkflowTemplate[]) => void
  addExecution: (e: WorkflowExecution) => void
  updateExecution: (id: string, patch: Partial<WorkflowExecution>) => void
  updateStepStatus: (executionId: string, stepId: string, status: StepStatus) => void
  appendStreamLine: (executionId: string, line: string) => void
}
```

---

## 7. Test Plan

**Target:** >= 25 tests

```
src/renderer/src/components/workflow/__tests__/
├── WorkflowBuilder.test.tsx         (6+ tests)
│   ├── renders step list from template
│   ├── Add Step adds step with default config
│   ├── Remove Step removes step and cleans dependsOn references
│   ├── update step field updates template state
│   ├── Save button calls rpc workflow.template.update
│   └── DAGPreview toggles on/off
├── DAGPreview.test.tsx              (5+ tests)
│   ├── linear deps => wave 0 and wave 1 nodes
│   ├── parallel (no deps) => all in wave 0
│   ├── creates edges for each dependency
│   ├── selected step => blue border style
│   └── empty steps => renders empty ReactFlow
├── ExecutionMonitor.test.tsx        (5+ tests)
│   ├── renders wave groups correctly
│   ├── shows running step with streaming output area
│   ├── Cancel button calls RPC
│   ├── completed status => green icon
│   └── pending status => grey icon
└── hooks/__tests__/useWorkflow.test.ts (5+ tests)
    ├── saveTemplate calls create (no templateId)
    ├── saveTemplate calls update (with templateId)
    ├── runWorkflow calls workflow.execute
    ├── runWorkflow adds execution to store
    └── updateTemplate merges patches correctly
```

---

## 8. Phu thuoc va Thu tu

**Prerequisite:** `@xyflow/react` phai duoc install

**Cach kiem tra va install:**
```bash
cat package.json | grep xyflow  # kiem tra
npm install @xyflow/react       # neu chua co
```

**Sau khi implement SOL-FE-V6-004:**
- `WorkspaceLayout` (`SOL-FE-V6-002`) render `WorkflowMonitor` trong Workflows tab
- Admin SPA co the them `/admin/workflows` page dung `WorkflowBuilder`
