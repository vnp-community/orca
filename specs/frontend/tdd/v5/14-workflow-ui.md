# TDD-FE-14: Workflow Builder & Monitor

**Document:** TDD-FE-14 (NEW — v5.0)
**Version:** 1.0
**Date:** 2026-07-28
**Domain:** Workflow UI — template builder, DAG preview, execution monitor, step streaming
**Feature:** F36
**ADR:** ADR-009
**HLD Ref:** C3.11c
**Backend TDD:** TDD-17
**Source files (to create):**
- `src/renderer/src/components/workflow/WorkflowBuilder.tsx`
- `src/renderer/src/components/workflow/StepEditor.tsx`
- `src/renderer/src/components/workflow/DAGPreview.tsx`
- `src/renderer/src/components/workflow/ExecutionMonitor.tsx`
- `src/renderer/src/components/workflow/StepStatusBadge.tsx`
- `src/renderer/src/hooks/useWorkflow.ts`

> **Status: ❌ TODO** — v5.0 proposed

---

## 1. WorkflowBuilder Layout

```
┌──────────────────────────────────────────────────────────────────────────┐
│ Workflow Builder: [Template Name]            [Preview DAG] [Save] [Run]  │
├──────────────────────────────────────────────────────────────────────────┤
│  STEPS PANEL              │  STEP EDITOR                │  DAG PREVIEW   │
│  (drag to reorder)        │                             │  (read-only)   │
│                           │  Step Type: [Agent ▼]       │                │
│  [●] 1. Setup DB          │  Name: [Run migrations]     │  ●  setup-db   │
│      → agent (dev-srv1)   │  Server: [dev-srv1 ▼]       │  │             │
│                           │  Prompt: [textarea...]       │  ▼             │
│  [●] 2. Run Tests         │  Depends On: [Step 1 ✓]     │  ● run-tests   │
│      → shell (dev-srv1)   │  Continue on error: [ ]     │  │             │
│                           │  Timeout: [30 min ▼]        │  ▼             │
│  [+] Add Step             │                             │  ● notify      │
│                           │  [Save Step] [Delete]       │                │
├───────────────────────────┴─────────────────────────────┴────────────────┤
│ Inheritance: [Parent Template: None ▼] [Scope: Personal ▼]               │
└──────────────────────────────────────────────────────────────────────────┘
```

---

## 2. WorkflowBuilder Component

```typescript
// src/renderer/src/components/workflow/WorkflowBuilder.tsx

export function WorkflowBuilder({ templateId }: { templateId?: string }) {
  const { template, updateTemplate, saveTemplate, runWorkflow } = useWorkflow(templateId)
  const [selectedStepId, setSelectedStepId] = useState<string | null>(null)
  const [showDagPreview, setShowDagPreview] = useState(true)

  const addStep = () => {
    const newStep: WorkflowStep = {
      id: randomId(),
      type: 'agent',
      name: `Step ${(template?.steps.length ?? 0) + 1}`,
      serverSpec: 'project:current',
      config: { type: 'agent', prompt: '', worktreePath: '.' },
      dependsOn: [],
    }
    updateTemplate({ steps: [...(template?.steps ?? []), newStep] })
    setSelectedStepId(newStep.id)
  }

  const updateStep = (stepId: string, patch: Partial<WorkflowStep>) => {
    updateTemplate({
      steps: template!.steps.map(s => s.id === stepId ? { ...s, ...patch } : s)
    })
  }

  const removeStep = (stepId: string) => {
    updateTemplate({
      steps: template!.steps
        .filter(s => s.id !== stepId)
        .map(s => ({
          ...s,
          dependsOn: s.dependsOn?.filter(d => d !== stepId),
        }))
    })
    if (selectedStepId === stepId) setSelectedStepId(null)
  }

  return (
    <div className="workflow-builder">
      <WorkflowBuilderHeader
        name={template?.name ?? ''}
        onNameChange={name => updateTemplate({ name })}
        onSave={saveTemplate}
        onRun={runWorkflow}
        showDag={showDagPreview}
        onToggleDag={() => setShowDagPreview(v => !v)}
      />
      <div className="workflow-builder-body">
        <StepList
          steps={template?.steps ?? []}
          selectedStepId={selectedStepId}
          onSelect={setSelectedStepId}
          onAdd={addStep}
          onReorder={(from, to) => reorderSteps(from, to)}
        />
        {selectedStepId && (
          <StepEditor
            step={template!.steps.find(s => s.id === selectedStepId)!}
            allSteps={template!.steps}
            onUpdate={patch => updateStep(selectedStepId, patch)}
            onDelete={() => removeStep(selectedStepId)}
          />
        )}
        {showDagPreview && <DAGPreview steps={template?.steps ?? []} selectedStepId={selectedStepId} />}
      </div>
      <WorkflowInheritanceBar
        parentTemplateId={template?.templateId}
        scope={template?.scope}
        onInheritanceChange={...}
      />
    </div>
  )
}
```

---

## 3. DAGPreview — React Flow

```typescript
// src/renderer/src/components/workflow/DAGPreview.tsx
// Uses @xyflow/react (React Flow v12)

import { ReactFlow, Node, Edge, Background, Controls } from '@xyflow/react'

interface DAGPreviewProps {
  steps: WorkflowStep[]
  selectedStepId: string | null
}

export function DAGPreview({ steps, selectedStepId }: DAGPreviewProps) {
  const { nodes, edges } = useMemo(() => buildDAGLayout(steps), [steps])

  return (
    <div className="dag-preview h-full">
      <ReactFlow
        nodes={nodes.map(n => ({
          ...n,
          selected: n.id === selectedStepId,
          style: {
            background: n.id === selectedStepId ? '#dbeafe' : '#f8fafc',
            border: n.id === selectedStepId ? '2px solid #3b82f6' : '1px solid #e2e8f0',
            borderRadius: 8,
          }
        }))}
        edges={edges}
        fitView
        nodesDraggable={false}  // preview only
        nodesConnectable={false}
        elementsSelectable={false}
      >
        <Background />
        <Controls showInteractive={false} />
      </ReactFlow>
    </div>
  )
}

function buildDAGLayout(steps: WorkflowStep[]): { nodes: Node[]; edges: Edge[] } {
  // Simple layered layout: compute wave/column for each step
  const nodes: Node[] = []
  const edges: Edge[] = []
  const waveMap = new Map<string, number>()

  // Topological sort to assign wave numbers
  const visited = new Set<string>()
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

  // Group by wave → calculate x/y
  const waveGroups = new Map<number, string[]>()
  for (const [id, wave] of waveMap) {
    if (!waveGroups.has(wave)) waveGroups.set(wave, [])
    waveGroups.get(wave)!.push(id)
  }

  for (const [wave, ids] of waveGroups) {
    ids.forEach((id, idx) => {
      const step = steps.find(s => s.id === id)!
      nodes.push({
        id,
        position: { x: wave * 200, y: idx * 80 },
        data: { label: `${step.name}\n(${step.type})` },
        type: 'default',
      })
    })
  }

  for (const step of steps) {
    for (const dep of step.dependsOn ?? []) {
      edges.push({ id: `${dep}-${step.id}`, source: dep, target: step.id, animated: true })
    }
  }

  return { nodes, edges }
}
```

---

## 4. ExecutionMonitor Component

```typescript
// src/renderer/src/components/workflow/ExecutionMonitor.tsx

// Shows real-time execution status + step outputs

// Layout:
// ┌──────────────────────────────────────────────────────────────────┐
// │ Execution: deploy-prod-2026-07-28  [Running ●]    [Cancel]      │
// │ Started: 2 min ago  Triggered by: binh@org                      │
// ├──────────────────────────────────────────────────────────────────┤
// │ Wave 0 (parallel):                                               │
// │   ✅ Setup DB        completed 1m23s  (linux-srv1)               │
// │   ✅ Warmup Cache    completed 0m45s  (linux-srv2)               │
// │                                                                  │
// │ Wave 1 (sequential):                                             │
// │   🔄 Run Tests       running 0m30s...                            │
// │      ▼ Output:                                                   │
// │      Test suite: auth (34/34 pass)                               │
// │      Test suite: api (12/15 pass)...                             │
// │   ⏳ Deploy          pending                                     │
// │   ⏳ Notify          pending                                     │
// └──────────────────────────────────────────────────────────────────┘

export function ExecutionMonitor({ executionId }: { executionId: string }) {
  const { execution, stepStatuses, streamingOutput } = useWorkflowExecution(executionId)

  if (!execution) return <Skeleton className="h-40" />

  const waves = groupStepsByWave(execution.definition.steps, stepStatuses)

  return (
    <div className="execution-monitor">
      <ExecutionHeader execution={execution} onCancel={() => cancelExecution(executionId)} />
      <div className="steps-list space-y-4">
        {waves.map((wave, waveIdx) => (
          <div key={waveIdx} className="wave-group">
            <div className="wave-header text-xs text-muted-foreground mb-2">
              Wave {waveIdx} {wave.length > 1 ? `(${wave.length} parallel)` : ''}
            </div>
            {wave.map(({ step, status }) => (
              <StepMonitorRow key={step.id} step={step} status={status}>
                {status === 'running' && streamingOutput[executionId]?.length > 0 && (
                  <StepOutputStream lines={streamingOutput[executionId]} />
                )}
              </StepMonitorRow>
            ))}
          </div>
        ))}
      </div>
    </div>
  )
}
```

---

## 5. useWorkflow Hook

```typescript
// src/renderer/src/hooks/useWorkflow.ts

export function useWorkflow(templateId?: string) {
  const { templates, executions } = useAppStore(s => ({
    templates: s.templates,
    executions: s.executions,
  }))

  const template = templateId ? templates.find(t => t.id === templateId) : null
  const [localTemplate, setLocalTemplate] = useState<Partial<WorkflowDefinition>>(template ?? {})

  const updateTemplate = useCallback((patch: Partial<WorkflowDefinition>) => {
    setLocalTemplate(prev => ({ ...prev, ...patch }))
  }, [])

  const saveTemplate = useCallback(async () => {
    if (templateId) {
      await rpc.call('workflow.template.update', { templateId, ...localTemplate })
    } else {
      await rpc.call('workflow.template.create', localTemplate)
    }
    toast.success('Workflow saved')
  }, [templateId, localTemplate])

  const runWorkflow = useCallback(async (inputs?: Record<string, unknown>) => {
    if (!templateId) {
      toast.error('Save workflow first')
      return
    }
    const result = await rpc.call('workflow.execute', { templateId, inputs }) as { id: string }
    useAppStore.getState().addExecution({ id: result.id, status: 'running', .../* rest */ } as any)
    toast.success('Workflow started')
    return result.id
  }, [templateId])

  return { template: localTemplate, templates, executions, updateTemplate, saveTemplate, runWorkflow }
}
```

---

## 6. Test Coverage

```
src/renderer/src/components/workflow/__tests__/
├── WorkflowBuilder.test.tsx
│   ├── adds step with correct defaults
│   ├── removes step and cleans dependsOn references
│   ├── updates step field
│   └── save calls rpc.call('workflow.template.update')
├── DAGPreview.test.tsx
│   ├── linear deps → wave 0 and wave 1 nodes
│   ├── parallel (no deps) → all in wave 0
│   ├── creates edges for each dependency
│   └── selected step → blue highlight class
├── ExecutionMonitor.test.tsx
│   ├── renders wave groups correctly
│   ├── shows streaming output for running steps
│   ├── Cancel button calls cancelExecution
│   └── completed status → ✅ icon
└── hooks/__tests__/useWorkflow.test.ts
    ├── saveTemplate calls correct RPC (create vs update)
    ├── runWorkflow calls workflow.execute and adds to store
    └── updateTemplate merges patches correctly
```

**Target:** ≥ 25 tests
