# TASK-V5-19: WorkflowBuilder + StepList + useWorkflow

**Order:** 19 | **Prerequisite:** TASK-V5-17, TASK-V5-18 | **Tests:** 9

---

## Files Cần Tạo

### 1. `src/renderer/src/hooks/useWorkflow.ts`

```typescript
export function useWorkflow(templateId?: string) {
  const { templates, executions } = useAppStore(s => ({
    templates:  s.templates,
    executions: s.executions,
  }))
  const template = templateId ? templates.find(t => t.id === templateId) ?? null : null
  const [local, setLocal] = useState<Partial<WorkflowDefinition>>(template ?? {})

  const updateTemplate = useCallback((patch: Partial<WorkflowDefinition>) => {
    setLocal(prev => ({ ...prev, ...patch }))
  }, [])

  const addStep = useCallback(() => {
    const newStep: WorkflowStep = {
      id: `step-${Date.now()}`, type: 'agent', name: `Step ${(local.steps?.length ?? 0) + 1}`,
      serverSpec: 'project:current', config: { type: 'agent', prompt: '', worktreePath: '.' },
      dependsOn: [], continueOnError: false, timeout: 1800,
    }
    setLocal(prev => ({ ...prev, steps: [...(prev.steps ?? []), newStep] }))
    return newStep.id
  }, [local.steps])

  const removeStep = useCallback((stepId: string) => {
    setLocal(prev => ({
      ...prev,
      steps: (prev.steps ?? [])
        .filter(s => s.id !== stepId)
        .map(s => ({ ...s, dependsOn: s.dependsOn.filter(d => d !== stepId) }))
    }))
  }, [])

  const updateStep = useCallback((stepId: string, patch: Partial<WorkflowStep>) => {
    setLocal(prev => ({
      ...prev,
      steps: (prev.steps ?? []).map(s => s.id === stepId ? { ...s, ...patch } : s)
    }))
  }, [])

  const saveTemplate = useCallback(async () => {
    if (templateId) {
      await callRuntimeRpc('workflow.template.update', { templateId, ...local })
    } else {
      const created = await callRuntimeRpc('workflow.template.create', local) as WorkflowDefinition
      useAppStore.getState().addTemplate(created)
    }
    toast.success('Workflow saved')
  }, [templateId, local])

  const runWorkflow = useCallback(async (inputs?: Record<string, unknown>) => {
    if (!templateId) { toast.error('Save workflow first'); return null }
    const result = await callRuntimeRpc('workflow.execute', { templateId, inputs }) as { id: string }
    return result.id
  }, [templateId])

  return { template: local, templates, executions, addStep, removeStep, updateStep, updateTemplate, saveTemplate, runWorkflow }
}
```

### 2. `src/renderer/src/components/workflow/StepList.tsx`

```typescript
import { DndContext, closestCenter, type DragEndEvent } from '@dnd-kit/core'
import { SortableContext, verticalListSortingStrategy, useSortable } from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import type { WorkflowStep } from '@shared/workflow-types'

function SortableStep({ step, isSelected, onSelect }) {
  const { attributes, listeners, setNodeRef, transform, transition } = useSortable({ id: step.id })
  return (
    <div
      ref={setNodeRef}
      style={{ transform: CSS.Transform.toString(transform), transition }}
      className={`flex items-center gap-2 px-2 py-1.5 cursor-pointer hover:bg-accent/50 rounded ${isSelected ? 'bg-accent' : ''}`}
      onClick={() => onSelect(step.id)}
      data-testid={`step-item-${step.id}`}
    >
      <span {...attributes} {...listeners} className="cursor-grab text-muted-foreground">⠿</span>
      <span className="text-xs font-mono text-muted-foreground">{step.type}</span>
      <span className="text-sm truncate">{step.name}</span>
    </div>
  )
}

export function StepList({ steps, selectedStepId, onSelect, onAdd, onReorder }) {
  const handleDragEnd = (e: DragEndEvent) => {
    const { active, over } = e
    if (!over || active.id === over.id) return
    const from = steps.findIndex(s => s.id === active.id)
    const to   = steps.findIndex(s => s.id === over.id)
    if (from !== -1 && to !== -1) onReorder(from, to)
  }

  return (
    <div className="step-list" data-testid="step-list">
      <DndContext collisionDetection={closestCenter} onDragEnd={handleDragEnd}>
        <SortableContext items={steps.map(s => s.id)} strategy={verticalListSortingStrategy}>
          {steps.map(step => (
            <SortableStep key={step.id} step={step} isSelected={step.id === selectedStepId} onSelect={onSelect} />
          ))}
        </SortableContext>
      </DndContext>
      <button
        className="flex items-center gap-1 px-2 py-1 text-sm text-muted-foreground hover:text-foreground mt-1"
        onClick={onAdd}
        data-testid="add-step-btn"
      >
        <Plus size={12} /> Add Step
      </button>
    </div>
  )
}
```

### 3. `src/renderer/src/components/workflow/WorkflowBuilder.tsx`

```typescript
import { useState } from 'react'
import { useWorkflow } from '../../hooks/useWorkflow'
import { StepList } from './StepList'
import { lazy, Suspense } from 'react'
import { arrayMove } from '@dnd-kit/sortable'

const DAGPreview = lazy(() => import('../shared/DAGPreview').then(m => ({ default: m.DAGPreview })))
const StepEditor = lazy(() => import('./StepEditor').then(m => ({ default: m.StepEditor })))

export function WorkflowBuilder({ templateId }: { templateId?: string }) {
  const { template, addStep, removeStep, updateStep, updateTemplate, saveTemplate, runWorkflow } = useWorkflow(templateId)
  const [selectedStepId, setSelectedStep] = useState<string | null>(null)
  const [showDAG, setShowDAG]             = useState(true)

  const handleReorder = (from: number, to: number) => {
    const reordered = arrayMove(template.steps ?? [], from, to)
    updateTemplate({ steps: reordered })
  }

  return (
    <div className="workflow-builder flex flex-col h-full" data-testid="workflow-builder">
      {/* Header */}
      <div className="flex items-center gap-2 p-2 border-b">
        <input
          value={template.name ?? ''}
          onChange={e => updateTemplate({ name: e.target.value })}
          placeholder="Workflow name..."
          className="flex-1 text-base font-semibold border-0 focus:outline-none bg-transparent"
          data-testid="workflow-name-input"
        />
        <Button size="sm" variant="outline" onClick={() => setShowDAG(v => !v)}>
          {showDAG ? 'Hide DAG' : 'Preview DAG'}
        </Button>
        <Button size="sm" variant="outline" onClick={saveTemplate} data-testid="save-workflow-btn">Save</Button>
        <Button size="sm" onClick={() => runWorkflow()} data-testid="run-workflow-btn">Run</Button>
      </div>

      <div className="flex flex-1 overflow-hidden">
        {/* Steps panel */}
        <div className="w-52 border-r overflow-y-auto p-2">
          <StepList
            steps={template.steps ?? []}
            selectedStepId={selectedStepId}
            onSelect={setSelectedStep}
            onAdd={() => { const id = addStep(); setSelectedStep(id) }}
            onReorder={handleReorder}
          />
        </div>

        {/* Step editor */}
        <div className="flex-1 overflow-auto p-2">
          <Suspense>
            {selectedStepId && template.steps && (
              <StepEditor
                step={template.steps.find(s => s.id === selectedStepId)!}
                allSteps={template.steps}
                onUpdate={patch => updateStep(selectedStepId, patch)}
                onDelete={() => { removeStep(selectedStepId); setSelectedStep(null) }}
              />
            )}
          </Suspense>
        </div>

        {/* DAG preview */}
        {showDAG && (
          <div className="w-72 border-l">
            <Suspense>
              <DAGPreview steps={template.steps ?? []} selectedStepId={selectedStepId} />
            </Suspense>
          </div>
        )}
      </div>
    </div>
  )
}
```

### 4. `src/renderer/src/components/workflow/StepEditor.tsx`

```typescript
// Form for editing a single step: type, name, server, prompt/command, dependsOn, timeout, continueOnError

export function StepEditor({ step, allSteps, onUpdate, onDelete }) {
  return (
    <div className="step-editor space-y-3" data-testid="step-editor">
      <div className="flex items-center justify-between">
        <Input value={step.name} onChange={e => onUpdate({ name: e.target.value })} className="font-medium" />
        <Button size="sm" variant="ghost" className="text-red-600" onClick={onDelete}>Delete</Button>
      </div>
      <div>
        <Label>Type</Label>
        <Select value={step.type} onValueChange={t => onUpdate({ type: t as any })}>
          <SelectTrigger><SelectValue /></SelectTrigger>
          <SelectContent>
            {['agent','shell','notify','approval'].map(t => <SelectItem key={t} value={t}>{t}</SelectItem>)}
          </SelectContent>
        </Select>
      </div>
      {step.type === 'agent' && (
        <div>
          <Label>Prompt</Label>
          <Textarea
            value={(step.config as any).prompt ?? ''}
            onChange={e => onUpdate({ config: { ...step.config, prompt: e.target.value } })}
            rows={4}
          />
        </div>
      )}
      <div>
        <Label>Depends On</Label>
        {allSteps.filter(s => s.id !== step.id).map(s => (
          <label key={s.id} className="flex items-center gap-2 text-sm">
            <input
              type="checkbox"
              checked={step.dependsOn.includes(s.id)}
              onChange={e => onUpdate({
                dependsOn: e.target.checked
                  ? [...step.dependsOn, s.id]
                  : step.dependsOn.filter(d => d !== s.id)
              })}
            />
            {s.name}
          </label>
        ))}
      </div>
    </div>
  )
}
```

---

## Tests (9 total)

```
__tests__/workflow/WorkflowBuilder.test.tsx  (5 tests)
  adds step with correct defaults | removes step and cleans dependsOn
  updates step field | save calls RPC | shows DAGPreview when toggle on

hooks/__tests__/useWorkflow.test.ts          (4 tests)
  saveTemplate calls workflow.template.create for new
  saveTemplate calls workflow.template.update for existing
  runWorkflow calls workflow.execute
  removeStep cleans dependsOn of other steps
```

## Acceptance Criteria

- [x] `WorkflowBuilder` 3-panel layout (steps / editor / dag)
- [x] `StepList` drag-to-reorder with dnd-kit
- [x] Add step: generates unique id, selects it
- [x] Remove step: cleans `dependsOn` refs in other steps
- [x] Save: RPC called with correct method (create vs update)
- [x] 9/9 tests pass
