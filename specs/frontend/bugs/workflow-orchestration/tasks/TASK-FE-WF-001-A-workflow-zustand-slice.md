# TASK-FE-WF-001-A: Zustand Workflow Slice (`workflow.ts`)

**Domain:** workflow-orchestration  
**Solution Ref:** SOL-FE-WF-001B §Zustand Workflow Slice  
**Bug:** BUG-FE-WF-001  
**Priority:** 🔴 P0  
**Estimated:** 30 phút  
**Status:** ✅ DONE — Already implemented in codebase

---

## Mục tiêu

Tạo `workflow.ts` Zustand slice với đầy đủ state và actions cho templates + executions.

---

## Files cần tạo/sửa

- **TẠO MỚI:** `src/renderer/src/store/slices/workflow.ts`
- **MODIFY:** `src/renderer/src/store/index.ts` — register `createWorkflowSlice`

---

## Các bước thực thi

### Tạo `workflow.ts` với các types và slice

```typescript
// Types:
export type WorkflowDefinition = { id, name, scope, steps, templateId?, stepsCount, lastModified, ownerId }
export type WorkflowStep = { id, type: 'agent'|'shell'|'notify', name, serverSpec, config, dependsOn, continueOnError?, timeoutMinutes? }
export type WorkflowExecution = { id, templateId, name?, status: 'running'|'completed'|'failed'|'cancelled', startedAt, triggeredBy, definition?, stepStatuses? }

// Slice actions:
setTemplates(templates[])
addTemplate(template)
updateTemplate(templateId, patch)     // auto-sets lastModified = Date.now()
removeTemplate(templateId)
addExecution(execution)
updateExecutionStatus(executionId, status)
updateStep(executionId, stepId, status)
```

### Register trong `store/index.ts`

```typescript
import { createWorkflowSlice } from './slices/workflow'

export const useAppStore = create<AppState>()((...a) => ({
  // ... existing slices ...
  ...createWorkflowSlice(...a),
}))
```

---

## Verify

```bash
grep -n "createWorkflowSlice\|WorkflowDefinition" \
  src/renderer/src/store/slices/workflow.ts

grep -n "createWorkflowSlice" \
  src/renderer/src/store/index.ts
```

## Depends on
Không có

## Blocking
TASK-FE-WF-001-B, TASK-FE-WF-001-C, TASK-FE-WF-001-E
