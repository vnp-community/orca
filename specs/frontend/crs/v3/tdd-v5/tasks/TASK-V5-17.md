# TASK-V5-17: Install npm Dependencies + Workflow Slice

**Order:** 17 | **Prerequisite:** TASK-V5-02 | **Tests:** 0 (build verification)

---

## Mô tả

Cài đặt dependencies bắt buộc cho Workflow Builder (React Flow, dnd-kit) và tạo WorkflowSlice.

---

## Commands Cần Chạy

```bash
# Trong workspace root (orca/):
cd src/renderer

# React Flow v12 (DAG visualization)
npm install @xyflow/react@^12.0.0

# dnd-kit (drag-to-reorder steps)
npm install @dnd-kit/core@^6.0.0 @dnd-kit/sortable@^7.0.0 @dnd-kit/utilities@^3.0.0
```

---

## Files Cần Tạo

### `src/renderer/src/store/slices/workflow.ts`

```typescript
import type { WorkflowDefinition, WorkflowExecution, WorkflowExecutionStatus, StepStatus } from '@shared/workflow-types'

export type WorkflowSlice = {
  templates:       WorkflowDefinition[]
  executions:      WorkflowExecution[]
  stepStatuses:    Record<string, Record<string, StepStatus>>  // execId → stepId → status
  streamingOutput: Record<string, string[]>                    // execId → lines[]
  workflowLoading: boolean

  setTemplates(templates: WorkflowDefinition[]): void
  addTemplate(template: WorkflowDefinition): void
  updateTemplate(id: string, patch: Partial<WorkflowDefinition>): void
  removeTemplate(id: string): void
  addExecution(execution: WorkflowExecution): void
  updateExecutionStatus(execId: string, status: WorkflowExecutionStatus): void
  setStepStatus(execId: string, stepId: string, status: StepStatus): void
  appendStreamLine(execId: string, line: string): void
  clearStreamLines(execId: string): void
  setWorkflowLoading(v: boolean): void
}

export function createWorkflowSlice(set): WorkflowSlice {
  return {
    templates:       [],
    executions:      [],
    stepStatuses:    {},
    streamingOutput: {},
    workflowLoading: false,

    setTemplates: (t)   => set(s => { s.templates = t }),
    addTemplate:  (t)   => set(s => { s.templates.push(t) }),
    updateTemplate: (id, patch) => set(s => {
      const idx = s.templates.findIndex((t: WorkflowDefinition) => t.id === id)
      if (idx !== -1) Object.assign(s.templates[idx], patch)
    }),
    removeTemplate: (id) => set(s => { s.templates = s.templates.filter((t: WorkflowDefinition) => t.id !== id) }),
    addExecution:  (e)   => set(s => { s.executions.push(e) }),
    updateExecutionStatus: (execId, status) => set(s => {
      const e = s.executions.find((ex: WorkflowExecution) => ex.id === execId)
      if (e) e.status = status
    }),
    setStepStatus: (execId, stepId, status) => set(s => {
      if (!s.stepStatuses[execId]) s.stepStatuses[execId] = {}
      s.stepStatuses[execId][stepId] = status
    }),
    appendStreamLine: (execId, line) => set(s => {
      if (!s.streamingOutput[execId]) s.streamingOutput[execId] = []
      s.streamingOutput[execId].push(line)
    }),
    clearStreamLines: (execId) => set(s => { s.streamingOutput[execId] = [] }),
    setWorkflowLoading: (v) => set(s => { s.workflowLoading = v }),
  }
}
```

---

## Files Cần Sửa

`store/index.ts` → register `createWorkflowSlice`

---

## Verification

```bash
# Verify packages installed:
ls src/renderer/node_modules/@xyflow/react/package.json
ls src/renderer/node_modules/@dnd-kit/core/package.json

# Verify no TypeScript errors:
npx tsc --noEmit -p src/renderer/tsconfig.json 2>&1 | head -20
```

## Acceptance Criteria

- [x] `@xyflow/react` installed (check node_modules)
- [x] `@dnd-kit/core`, `@dnd-kit/sortable`, `@dnd-kit/utilities` installed
- [x] `WorkflowSlice` registered trong store
- [x] `npx tsc --noEmit` passes (no new errors)
