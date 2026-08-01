# TASK-FE-WF-001-D: `WorkflowBuilder` + `useWorkflow` hook (BL-WF-01)

**Domain:** workflow-orchestration  
**Solution Ref:** SOL-FE-WF-001 §Components + SOL-FE-WF-001B §useWorkflow  
**Bug:** BUG-FE-WF-001  
**Priority:** 🟡 P2  
**Estimated:** 60 phút  
**Status:** ✅ DONE — Already implemented in codebase

---

## Mục tiêu

1. Tạo `useWorkflow` hook — load/update/save/run templates
2. Tạo `WorkflowBuilder` — 3-panel layout: StepList + StepEditor + DAGPreview

---

## Files cần tạo

- **TẠO MỚI:** `src/renderer/src/hooks/useWorkflow.ts`
- **TẠO MỚI:** `src/renderer/src/components/workflow/workflow-builder.tsx`

---

## Bước 1: `useWorkflow.ts`

```typescript
export function useWorkflow(templateId?: string) {
  // Load từ store.templates, local edit state
  const [localTemplate, setLocalTemplate] = useState<Partial<WorkflowDefinition>>(template ?? {})

  const updateTemplate = (patch) => setLocalTemplate(prev => ({ ...prev, ...patch }))

  const saveTemplate = async () => {
    if (templateId) await rpc.call('workflow.template.update', { templateId, ...localTemplate })
    else await rpc.call('workflow.template.create', localTemplate)
    toast.success('Workflow saved')
  }

  const runWorkflow = async (inputs?) => {
    const result = await rpc.call('workflow.execute', { templateId, inputs })
    store.addExecution({ id: result.id, status: 'running', ... })
    return result.id
  }

  return { template: localTemplate, templates, executions, updateTemplate, saveTemplate, runWorkflow }
}
```

## Bước 2: `workflow-builder.tsx`

Layout 3-panel:
```
[Header: name input | Preview DAG toggle | Save | Run]
[Body]:
  [StepList 200px — drag reorder, Add Step button]
  [StepEditor flex — khi selectedStepId set]
  [DAGPreview — khi showDagPreview]
[Footer: WorkflowInheritanceBar]
```

Logic:
- `addStep()` → tạo step mới với `id: randomId()`, `type: 'agent'`, default config
- `updateStep(stepId, patch)` → map over `template.steps`
- `removeStep(stepId)` → filter + clean `dependsOn` references

---

## Verify

```bash
grep -n "useWorkflow\|WorkflowBuilder" \
  src/renderer/src/hooks/useWorkflow.ts \
  src/renderer/src/components/workflow/workflow-builder.tsx
```

## Test

```typescript
// WorkflowBuilder.test.tsx
// - addStep creates step with correct defaults
// - removeStep cleans dependsOn references in other steps
// - updateStep patches step by id
// - save calls rpc.call('workflow.template.update') with localTemplate
```

## Depends on
TASK-FE-WF-001-A (slice), TASK-FE-WF-001-B (StepEditor), TASK-FE-WF-001-C (DAGPreview)

## Blocking
TASK-FE-WF-001-E (ExecutionMonitor)
