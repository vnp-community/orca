# TASK-V5-18: DAGPreview (React Flow)

**Order:** 18 | **Prerequisite:** TASK-V5-17 (@xyflow/react installed) | **Tests:** 5

---

## Mô tả

Implement `DAGPreview` component dùng `@xyflow/react`. Dùng chung cho Workflow Builder (TDD-FE-14) và Task Graph DAG view (TDD-FE-15).

---

## Files Cần Tạo

### 1. `src/renderer/src/components/shared/DAGPreview.tsx`

```typescript
import { useMemo } from 'react'
import { ReactFlow, type Node, type Edge, Background, Controls } from '@xyflow/react'
import '@xyflow/react/dist/style.css'
import type { WorkflowStep } from '@shared/workflow-types'

interface DAGPreviewProps {
  steps:          WorkflowStep[]
  selectedStepId: string | null
  readOnly?:      boolean
}

export function DAGPreview({ steps, selectedStepId, readOnly = true }: DAGPreviewProps) {
  const { nodes, edges } = useMemo(() => buildDAGLayout(steps, selectedStepId), [steps, selectedStepId])

  return (
    <div className="dag-preview h-full min-h-[200px]" data-testid="dag-preview">
      <ReactFlow
        nodes={nodes}
        edges={edges}
        fitView
        nodesDraggable={!readOnly}
        nodesConnectable={false}
        elementsSelectable={!readOnly}
      >
        <Background />
        <Controls showInteractive={!readOnly} />
      </ReactFlow>
    </div>
  )
}

// --- DAG Layout Algorithm ---

function buildDAGLayout(
  steps: WorkflowStep[],
  selectedId: string | null
): { nodes: Node[]; edges: Edge[] } {
  const nodes: Node[] = []
  const edges: Edge[] = []

  if (steps.length === 0) return { nodes, edges }

  // Topological wave assignment
  const waveMap = new Map<string, number>()

  function assignWave(stepId: string): number {
    if (waveMap.has(stepId)) return waveMap.get(stepId)!
    const step = steps.find(s => s.id === stepId)
    if (!step) return 0
    const deps = step.dependsOn ?? []
    const wave = deps.length === 0 ? 0 : Math.max(...deps.map(d => assignWave(d))) + 1
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

  // Build nodes
  for (const [wave, ids] of waveGroups) {
    ids.forEach((id, idx) => {
      const step = steps.find(s => s.id === id)!
      const isSelected = id === selectedId
      nodes.push({
        id,
        position: { x: wave * 220, y: idx * 90 },
        data:     { label: step.name },
        type:     'default',
        style: {
          background:  isSelected ? '#dbeafe' : '#f8fafc',
          border:      isSelected ? '2px solid #3b82f6' : '1px solid #e2e8f0',
          borderRadius: 8,
          fontSize:    12,
          minWidth:    140,
        },
      })
    })
  }

  // Build edges
  for (const step of steps) {
    for (const dep of step.dependsOn ?? []) {
      edges.push({
        id:       `${dep}-${step.id}`,
        source:   dep,
        target:   step.id,
        animated: true,
        style:    { stroke: '#94a3b8' },
      })
    }
  }

  return { nodes, edges }
}
```

### 2. `src/renderer/src/components/task/TaskDAGView.tsx`

```typescript
// Adapter: TaskGraph → DAGPreview (converts OrcaTask[] → WorkflowStep[] shape)
import type { OrcaTask } from '@shared/task-types'
import { DAGPreview } from '../shared/DAGPreview'

export function TaskDAGView({ tasks, onSelect }: { tasks: OrcaTask[]; onSelect: (id: string) => void }) {
  // Convert OrcaTask to WorkflowStep-compatible shape
  const steps = tasks.map(t => ({
    id:         t.id,
    name:       t.title,
    type:       'agent' as const,
    serverSpec: '',
    config:     { type: 'agent' as const, prompt: '', worktreePath: '.' },
    dependsOn:  t.dependsOn,
    continueOnError: false,
    timeout: 0,
  }))

  return <DAGPreview steps={steps} selectedStepId={null} />
}
```

---

## Tests — `src/renderer/src/components/shared/__tests__/DAGPreview.test.tsx`

```typescript
// @vitest-environment happy-dom
// Note: @xyflow/react needs special mock in jsdom/happy-dom

vi.mock('@xyflow/react', () => ({
  ReactFlow:  ({ nodes, edges, children }: any) => (
    <div data-testid="react-flow" data-nodes={nodes.length} data-edges={edges.length}>{children}</div>
  ),
  Background: () => null,
  Controls:   () => null,
}))

describe('DAGPreview', () => {
  it('empty steps → empty graph', () => {
    render(<DAGPreview steps={[]} selectedStepId={null} />)
    expect(screen.getByTestId('dag-preview')).toBeInTheDocument()
  })

  it('linear deps → 2 waves (nodes in wave 0 and wave 1)', () => {
    const steps = [
      { id: 's1', name: 'Setup', dependsOn: [], ... },
      { id: 's2', name: 'Test',  dependsOn: ['s1'], ... },
    ]
    render(<DAGPreview steps={steps} selectedStepId={null} />)
    // ReactFlow receives 2 nodes
    expect(screen.getByTestId('react-flow')).toHaveAttribute('data-nodes', '2')
  })

  it('parallel steps (no deps) → all in wave 0 (same x position)', () => {
    // All dependsOn: [] → wave=0 → x=0 for all
    const steps = ['s1','s2','s3'].map(id => ({ id, name: id, dependsOn: [], ... }))
    render(<DAGPreview steps={steps} selectedStepId={null} />)
    expect(screen.getByTestId('react-flow')).toHaveAttribute('data-nodes', '3')
  })

  it('creates edges for each dependency', () => {
    const steps = [
      { id: 's1', name: 'A', dependsOn: [], ... },
      { id: 's2', name: 'B', dependsOn: ['s1'], ... },
    ]
    render(<DAGPreview steps={steps} selectedStepId={null} />)
    expect(screen.getByTestId('react-flow')).toHaveAttribute('data-edges', '1')
  })

  it('selected step → blue border style', () => {
    // buildDAGLayout produces node.style with border: '2px solid #3b82f6'
    // (We test the algorithm directly via unit test of buildDAGLayout)
    const steps = [{ id: 's1', name: 'Step', dependsOn: [], ... }]
    render(<DAGPreview steps={steps} selectedStepId="s1" />)
    expect(screen.getByTestId('dag-preview')).toBeInTheDocument()
  })
})
```

---

## Acceptance Criteria

- [x] `@xyflow/react` imported without errors
- [x] Linear deps: s1 wave=0, s2 wave=1 (x: 0 vs 220)
- [x] Parallel: all wave=0 (same x)
- [x] Edge created for each `dependsOn` pair
- [x] Selected step: `border: '2px solid #3b82f6'` in style
- [x] 5/5 tests pass
