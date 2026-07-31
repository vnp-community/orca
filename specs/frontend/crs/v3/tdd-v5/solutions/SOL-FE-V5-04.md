# SOL-FE-V5-04: Workflow Builder & Monitor

**TDD Ref:** [TDD-FE-14](../../../tdd/14-workflow-ui.md)  
**Feature:** F36 | **ADR:** ADR-009 | **HLD:** C3.11c  
**Status:** ✅ DONE — Implemented via TASK-V5-17, TASK-V5-18, TASK-V5-19, TASK-V5-20  
**Dependency:** WorkspaceContext (SOL-FE-V5-02 phải implement trước)

---

## 1. Files Cần Tạo

| File | Loại | Mô tả |
|------|------|-------|
| `src/renderer/src/store/slices/workflow.ts` | Zustand Slice | templates, executions, streamingOutput |
| `src/renderer/src/components/workflow/WorkflowBuilder.tsx` | Component | 3-pane builder |
| `src/renderer/src/components/workflow/StepList.tsx` | Component | Drag-to-reorder step list |
| `src/renderer/src/components/workflow/StepEditor.tsx` | Component | Step detail form |
| `src/renderer/src/components/workflow/DAGPreview.tsx` | Component | React Flow DAG visualization |
| `src/renderer/src/components/workflow/ExecutionMonitor.tsx` | Component | Real-time execution status |
| `src/renderer/src/components/workflow/StepMonitorRow.tsx` | Component | Per-step status row |
| `src/renderer/src/components/workflow/StepStatusBadge.tsx` | Component | Status icon badge |
| `src/renderer/src/components/workflow/StepOutputStream.tsx` | Component | Streaming output display |
| `src/renderer/src/components/workflow/WorkflowInheritanceBar.tsx` | Component | Parent template + scope selector |
| `src/renderer/src/hooks/useWorkflow.ts` | Hook | CRUD + execute templates |
| `src/renderer/src/hooks/useWorkflowExecution.ts` | Hook | Real-time execution monitoring |

---

## 2. New Dependencies

```json
// package.json additions
{
  "@xyflow/react": "^12.0.0",
  "@dnd-kit/core": "^6.0.0",
  "@dnd-kit/sortable": "^7.0.0"
}
```

**@xyflow/react** — DAGPreview (cũng dùng cho TDD-FE-15 TaskGraph)  
**@dnd-kit** — drag-to-reorder steps trong StepList

---

## 3. Workflow Slice

```typescript
// src/renderer/src/store/slices/workflow.ts

export type WorkflowDefinition = {
  id:           string
  name:         string
  templateId?:  string                  // parent template ID (inheritance)
  scope:        'personal' | 'project' | 'company'
  scopeRefId?:  string
  steps:        WorkflowStep[]
}

export type WorkflowStep = {
  id:              string
  type:            'agent' | 'shell' | 'notify' | 'approval'
  name:            string
  serverSpec:      string               // 'project:current' | 'server:<id>'
  config:          AgentStepConfig | ShellStepConfig | NotifyStepConfig
  dependsOn:       string[]             // step IDs
  continueOnError: boolean
  timeout:         number               // seconds
}

export type WorkflowExecution = {
  id:         string
  templateId: string
  status:     'pending' | 'running' | 'completed' | 'failed' | 'cancelled'
  startedAt:  number
  endedAt?:   number
  triggeredBy: string
  definition: WorkflowDefinition        // snapshot at execution time
}

export type WorkflowSlice = {
  templates:       WorkflowDefinition[]
  executions:      WorkflowExecution[]
  stepStatuses:    Record<string, Record<string, StepStatus>>   // execId → stepId → status
  streamingOutput: Record<string, string[]>                     // execId → lines
  isLoading:       boolean

  setTemplates(templates: WorkflowDefinition[]): void
  addExecution(execution: WorkflowExecution): void
  updateExecutionStatus(execId: string, status: WorkflowExecution['status']): void
  setStepStatus(execId: string, stepId: string, status: StepStatus): void
  appendStreamLine(execId: string, line: string): void
}
```

---

## 4. DAGPreview — React Flow Integration

```typescript
// src/renderer/src/components/workflow/DAGPreview.tsx
// Xem TDD-FE-14 section 3 — buildDAGLayout() algorithm đã documented

// Key: @xyflow/react phải lazy-loaded (heavy bundle)
const DAGPreviewLazy = lazy(() => import('./DAGPreview'))

// Trong WorkflowBuilder:
{showDagPreview && (
  <Suspense fallback={<Skeleton className="h-full" />}>
    <DAGPreviewLazy steps={template?.steps ?? []} selectedStepId={selectedStepId} />
  </Suspense>
)}
```

---

## 5. useWorkflowExecution — Streaming

```typescript
// src/renderer/src/hooks/useWorkflowExecution.ts

export function useWorkflowExecution(executionId: string) {
  const { stepStatuses, streamingOutput } = useAppStore(s => ({
    stepStatuses:    s.stepStatuses[executionId] ?? {},
    streamingOutput: s.streamingOutput,
  }))

  // Subscribe to streaming events via IPC
  useEffect(() => {
    const unsubs = [
      // Step status changes
      window.api.on('workflow:stepStatus', ({ execId, stepId, status }) => {
        if (execId !== executionId) return
        useAppStore.getState().setStepStatus(execId, stepId, status)
      }),
      // Streaming output lines
      window.api.on('workflow:stepOutput', ({ execId, stepId, line }) => {
        if (execId !== executionId) return
        useAppStore.getState().appendStreamLine(execId, line)
      }),
      // Execution complete
      window.api.on('workflow:complete', ({ execId, status }) => {
        if (execId !== executionId) return
        useAppStore.getState().updateExecutionStatus(execId, status)
      }),
    ]
    return () => unsubs.forEach(u => u())
  }, [executionId])

  const execution = useAppStore(s => s.executions.find(e => e.id === executionId))

  return { execution, stepStatuses, streamingOutput }
}
```

---

## 6. StepList — Drag-and-Drop

```typescript
// src/renderer/src/components/workflow/StepList.tsx
// Uses @dnd-kit/sortable

import { DndContext, closestCenter } from '@dnd-kit/core'
import { SortableContext, verticalListSortingStrategy, useSortable } from '@dnd-kit/sortable'

export function StepList({ steps, selectedStepId, onSelect, onAdd, onReorder }) {
  const handleDragEnd = (event) => {
    const { active, over } = event
    if (!over || active.id === over.id) return
    const oldIdx = steps.findIndex(s => s.id === active.id)
    const newIdx = steps.findIndex(s => s.id === over.id)
    onReorder(oldIdx, newIdx)
  }

  return (
    <DndContext collisionDetection={closestCenter} onDragEnd={handleDragEnd}>
      <SortableContext items={steps.map(s => s.id)} strategy={verticalListSortingStrategy}>
        {steps.map(step => (
          <SortableStepItem key={step.id} step={step} selected={step.id === selectedStepId} onSelect={onSelect} />
        ))}
      </SortableContext>
    </DndContext>
  )
}
```

---

## 7. Files Cần Sửa (Additive)

| File | Thay đổi |
|------|---------|
| `src/renderer/src/store/index.ts` | Register `createWorkflowSlice` |
| `src/renderer/src/components/workspace/WorkspaceLayout.tsx` | Thêm `<WorkflowMonitorPanel />` vào workflows tab |
| `package.json` | Add `@xyflow/react`, `@dnd-kit/core`, `@dnd-kit/sortable` |

---

## 8. RPC Methods

| Method | Params | Return |
|--------|--------|--------|
| `workflow.template.list` | `{ scope?, scopeRefId? }` | `WorkflowDefinition[]` |
| `workflow.template.get` | `{ templateId }` | `WorkflowDefinition` |
| `workflow.template.create` | `WorkflowDefinition` | `WorkflowDefinition` |
| `workflow.template.update` | `{ templateId, ...patch }` | `WorkflowDefinition` |
| `workflow.template.delete` | `{ templateId }` | `void` |
| `workflow.execute` | `{ templateId, inputs? }` | `{ id: string }` |
| `workflow.cancel` | `{ executionId }` | `void` |
| `workflow.execution.get` | `{ executionId }` | `WorkflowExecution` |
| `workflow.execution.list` | `{ templateId?, limit? }` | `WorkflowExecution[]` |

---

## 9. Test Plan

```
src/renderer/src/components/workflow/__tests__/
├── WorkflowBuilder.test.tsx       (5 tests)
│   ├── adds step with correct defaults
│   ├── removes step and cleans dependsOn references
│   ├── updates step field
│   ├── save calls rpc (create vs update)
│   └── shows DAGPreview when toggle on
├── DAGPreview.test.tsx            (5 tests)
│   ├── linear deps → wave 0 and wave 1 nodes
│   ├── parallel (no deps) → all in wave 0
│   ├── creates edges for each dependency
│   ├── selected step → blue highlight
│   └── empty steps → empty graph
├── ExecutionMonitor.test.tsx      (5 tests)
│   ├── renders wave groups correctly
│   ├── shows streaming output for running steps
│   ├── Cancel button calls workflow.cancel RPC
│   ├── completed status → ✅ icon
│   └── failed status → ❌ icon
├── StepList.test.tsx              (4 tests)
│   ├── renders all steps
│   ├── click step → calls onSelect
│   ├── drag-and-drop reorders steps
│   └── add step button calls onAdd
└── hooks/__tests__/useWorkflow.test.ts  (7 tests)
    ├── saveTemplate calls workflow.template.create for new
    ├── saveTemplate calls workflow.template.update for existing
    ├── runWorkflow calls workflow.execute
    ├── runWorkflow shows error toast if no templateId
    ├── updateTemplate merges patches correctly
    ├── removeStep from dependsOn when step deleted
    └── addStep generates unique id
```

**Target:** ≥ 26 tests
