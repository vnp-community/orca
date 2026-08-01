# SOL-FE-WF-001: Implement Workflow Builder UI (WorkflowBuilder, TemplateLibrary, ExecutionMonitor)

## Bug Reference
- **Bug:** BUG-FE-WF-001
- **Mức độ:** 🔴 HIGH (Feature Missing)
- **TDD Reference:** TDD-FE-14 (Workflow Builder & Monitor — đầy đủ spec)

---

## Root Cause

Toàn bộ Workflow Builder UI (BL-WF-01 → BL-WF-03) chưa implement:
- Không có `WorkflowBuilder` component
- Không có `TemplateLibrary` (CRUD)
- Không có `ExecutionMonitor` (SSE real-time progress)
- Không có `useWorkflow` hook

---

## Giải pháp

TDD-FE-14 đã định nghĩa đầy đủ. Solution implement theo spec với đầy đủ code examples từ TDD.

---

### Component 1: `workflow-builder.tsx`

**File:** `src/renderer/src/components/workflow/workflow-builder.tsx` (TẠO MỚI)

Implement theo **TDD-FE-14 §2 WorkflowBuilder Component** (full code đã có trong spec):

```typescript
// src/renderer/src/components/workflow/workflow-builder.tsx
// Theo TDD-FE-14 §2 — đầy đủ implementation

import { useState } from 'react'
import { useWorkflow } from '@/hooks/useWorkflow'
import { StepEditor } from './step-editor'
import { DAGPreview } from './dag-preview'
import { WorkflowStep } from '@/store/slices/workflow'
import { randomId } from '@/lib/random-id'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Plus, Play, Save, GitBranch } from 'lucide-react'

export function WorkflowBuilder({ templateId }: { templateId?: string }) {
  const { template, updateTemplate, saveTemplate, runWorkflow } = useWorkflow(templateId)
  const [selectedStepId, setSelectedStepId] = useState<string | null>(null)
  const [showDagPreview, setShowDagPreview] = useState(true)

  const addStep = () => {
    const newStep: WorkflowStep = {
      id: randomId(),
      type: 'agent',
      name: `Step ${(template?.steps?.length ?? 0) + 1}`,
      serverSpec: 'project:current',
      config: { type: 'agent', prompt: '', worktreePath: '.' },
      dependsOn: [],
    }
    updateTemplate({ steps: [...(template?.steps ?? []), newStep] })
    setSelectedStepId(newStep.id)
  }

  const updateStep = (stepId: string, patch: Partial<WorkflowStep>) => {
    updateTemplate({
      steps: template!.steps.map(s => s.id === stepId ? { ...s, ...patch } : s),
    })
  }

  const removeStep = (stepId: string) => {
    updateTemplate({
      steps: template!.steps
        .filter(s => s.id !== stepId)
        .map(s => ({
          ...s,
          dependsOn: s.dependsOn?.filter(d => d !== stepId),
        })),
    })
    if (selectedStepId === stepId) setSelectedStepId(null)
  }

  const reorderSteps = (fromIdx: number, toIdx: number) => {
    const steps = [...(template?.steps ?? [])]
    const [removed] = steps.splice(fromIdx, 1)
    steps.splice(toIdx, 0, removed)
    updateTemplate({ steps })
  }

  return (
    <div className="workflow-builder flex flex-col h-full">
      {/* Header */}
      <div className="workflow-builder-header flex items-center gap-3 px-4 py-2 border-b">
        <Input
          value={template?.name ?? ''}
          onChange={e => updateTemplate({ name: e.target.value })}
          placeholder="Workflow template name..."
          className="max-w-48 text-sm font-medium"
        />
        <div className="ml-auto flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            className="gap-2"
            onClick={() => setShowDagPreview(v => !v)}
          >
            <GitBranch size={14} />
            {showDagPreview ? 'Hide DAG' : 'Preview DAG'}
          </Button>
          <Button variant="outline" size="sm" className="gap-2" onClick={saveTemplate}>
            <Save size={14} />
            Save
          </Button>
          <Button size="sm" className="gap-2" onClick={runWorkflow}>
            <Play size={14} />
            Run
          </Button>
        </div>
      </div>

      {/* Body: Steps | Editor | DAG */}
      <div className="workflow-builder-body flex flex-1 overflow-hidden">
        {/* Steps panel */}
        <div className="steps-panel w-56 border-r overflow-y-auto p-2 space-y-1">
          {(template?.steps ?? []).map((step, idx) => (
            <button
              key={step.id}
              className={`w-full text-left px-2 py-1.5 rounded text-sm hover:bg-muted transition-colors ${
                selectedStepId === step.id ? 'bg-muted font-medium' : ''
              }`}
              onClick={() => setSelectedStepId(step.id)}
            >
              <span className="text-muted-foreground mr-1">{idx + 1}.</span>
              {step.name}
              <div className="text-xs text-muted-foreground mt-0.5">{step.type}</div>
            </button>
          ))}
          <Button
            variant="ghost"
            size="sm"
            className="w-full gap-2 justify-start text-muted-foreground"
            onClick={addStep}
          >
            <Plus size={14} />
            Add Step
          </Button>
        </div>

        {/* Step editor */}
        {selectedStepId && template?.steps && (
          <div className="flex-1 overflow-y-auto border-r">
            <StepEditor
              step={template.steps.find(s => s.id === selectedStepId)!}
              allSteps={template.steps}
              onUpdate={patch => updateStep(selectedStepId, patch)}
              onDelete={() => removeStep(selectedStepId)}
            />
          </div>
        )}

        {/* DAG preview */}
        {showDagPreview && (
          <div className="w-64 overflow-hidden">
            <DAGPreview
              steps={template?.steps ?? []}
              selectedStepId={selectedStepId}
            />
          </div>
        )}
      </div>
    </div>
  )
}
```

---

### Component 2: `workflow-template-library.tsx`

**File:** `src/renderer/src/components/workflow/workflow-template-library.tsx` (TẠO MỚI)

```typescript
// BL-WF-01: Template CRUD UI
// BL-WF-03: Library discovery + share

import { useState, useCallback } from 'react'
import { useAppStore } from '@/store'
import { useShallow } from 'zustand/react/shallow'
import { WorkflowBuilder } from './workflow-builder'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Plus, Play, Share2, Trash2, Search } from 'lucide-react'
import { rpc } from '@/platform/rpc-client-interface'
import { toast } from 'sonner'
import { useNavigate } from '@/lib/navigation'

type WorkflowTemplate = {
  id: string
  name: string
  scope: 'personal' | 'team' | 'public'
  stepsCount: number
  lastModified: number
  ownerId: string
}

export function WorkflowTemplateLibrary() {
  const { templates } = useAppStore(
    useShallow(s => ({ templates: s.templates as WorkflowTemplate[] }))
  )
  const [search, setSearch] = useState('')
  const [selectedTemplateId, setSelectedTemplateId] = useState<string | null>(null)
  const [isCreating, setIsCreating] = useState(false)
  const navigate = useNavigate()

  const filteredTemplates = templates.filter(t =>
    t.name.toLowerCase().includes(search.toLowerCase())
  )

  const createNew = () => {
    setSelectedTemplateId(null)
    setIsCreating(true)
  }

  const openTemplate = (templateId: string) => {
    setSelectedTemplateId(templateId)
    setIsCreating(false)
  }

  const executeTemplate = async (templateId: string, templateName: string) => {
    try {
      const result = await rpc.call('workflow.execute', { templateId }) as { id: string }
      toast.success(`Workflow "${templateName}" started`)
      // Navigate to execution monitor
      navigate(`/workflows/executions/${result.id}`)
    } catch (err: any) {
      toast.error(`Failed to run workflow: ${err.message}`)
    }
  }

  const toggleShare = async (template: WorkflowTemplate) => {
    const newScope = template.scope === 'personal' ? 'team' : 'personal'
    try {
      await rpc.call('workflow.template.update', {
        templateId: template.id,
        scope: newScope,
      })
      toast.success(`Workflow "${template.name}" is now ${newScope}`)
      // Optimistic update
      useAppStore.getState().updateTemplate?.(template.id, { scope: newScope })
    } catch {
      toast.error('Failed to update sharing settings')
    }
  }

  const deleteTemplate = async (templateId: string, name: string) => {
    if (!confirm(`Delete workflow "${name}"?`)) return
    try {
      await rpc.call('workflow.template.delete', { templateId })
      toast.success(`Workflow "${name}" deleted`)
      if (selectedTemplateId === templateId) setSelectedTemplateId(null)
      useAppStore.getState().removeTemplate?.(templateId)
    } catch {
      toast.error('Failed to delete workflow')
    }
  }

  // If editing a template, show builder
  if (isCreating || selectedTemplateId) {
    return (
      <div className="flex flex-col h-full">
        <div className="flex items-center gap-2 p-2 border-b">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => { setIsCreating(false); setSelectedTemplateId(null) }}
          >
            ← Back to Library
          </Button>
        </div>
        <WorkflowBuilder templateId={selectedTemplateId ?? undefined} />
      </div>
    )
  }

  return (
    <div className="workflow-template-library p-4 space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold">Workflow Library</h2>
        <Button className="gap-2" onClick={createNew}>
          <Plus size={14} />
          New Template
        </Button>
      </div>

      {/* Search */}
      <div className="relative">
        <Search size={14} className="absolute left-2.5 top-2.5 text-muted-foreground" />
        <Input
          value={search}
          onChange={e => setSearch(e.target.value)}
          placeholder="Search templates..."
          className="pl-8 text-sm"
        />
      </div>

      {/* Template list */}
      {filteredTemplates.length === 0 ? (
        <div className="text-center py-8 text-muted-foreground text-sm">
          {search ? 'No templates match your search' : 'No workflow templates yet. Click "New Template" to create one.'}
        </div>
      ) : (
        <div className="space-y-2">
          {filteredTemplates.map(template => (
            <div
              key={template.id}
              className="flex items-center gap-3 p-3 border rounded-lg bg-card hover:border-primary/50 transition-colors"
            >
              <div className="flex-1 min-w-0" onClick={() => openTemplate(template.id)} role="button">
                <div className="flex items-center gap-2">
                  <span className="font-medium text-sm truncate">{template.name}</span>
                  <Badge
                    variant={template.scope === 'personal' ? 'secondary' : 'default'}
                    className="text-xs"
                  >
                    {template.scope}
                  </Badge>
                </div>
                <p className="text-xs text-muted-foreground mt-0.5">
                  {template.stepsCount} step{template.stepsCount !== 1 ? 's' : ''}
                </p>
              </div>
              <div className="flex items-center gap-1 shrink-0">
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-7 w-7"
                  title="Run workflow"
                  onClick={() => executeTemplate(template.id, template.name)}
                >
                  <Play size={12} />
                </Button>
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-7 w-7"
                  title={template.scope === 'personal' ? 'Share with team' : 'Make private'}
                  onClick={() => toggleShare(template)}
                >
                  <Share2 size={12} />
                </Button>
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-7 w-7 text-destructive hover:text-destructive"
                  title="Delete template"
                  onClick={() => deleteTemplate(template.id, template.name)}
                >
                  <Trash2 size={12} />
                </Button>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
```

---

### Component 3: `workflow-execution-monitor.tsx`

**File:** `src/renderer/src/components/workflow/workflow-execution-monitor.tsx` (TẠO MỚI)

Implement theo **TDD-FE-14 §4 ExecutionMonitor Component**:

```typescript
// BL-WF-02: Execution monitor với wave progress (SSE real-time)
// Theo TDD-FE-14 §4 — full layout đã có trong spec

import { useWorkflowExecution } from '@/hooks/useWorkflow'
import { StepStatusBadge } from './step-status-badge'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { XCircle } from 'lucide-react'
import { rpc } from '@/platform/rpc-client-interface'
import { toast } from 'sonner'
import { formatDistanceToNow } from 'date-fns'

export function WorkflowExecutionMonitor({ executionId }: { executionId: string }) {
  const { execution, stepStatuses, streamingOutput } = useWorkflowExecution(executionId)

  if (!execution) return <Skeleton className="h-64" />

  const waves = groupStepsByWave(execution.definition?.steps ?? [], stepStatuses)

  const cancelExecution = async () => {
    try {
      await rpc.call('workflow.cancel', { executionId })
      toast.info('Workflow cancellation requested')
    } catch {
      toast.error('Failed to cancel workflow')
    }
  }

  return (
    <div className="execution-monitor p-4 space-y-4">
      {/* Execution header */}
      <div className="flex items-center justify-between">
        <div>
          <h3 className="font-semibold">{execution.name ?? executionId}</h3>
          <p className="text-xs text-muted-foreground">
            Started {formatDistanceToNow(execution.startedAt, { addSuffix: true })}
            {' · '}Triggered by {execution.triggeredBy}
          </p>
        </div>
        <div className="flex items-center gap-2">
          <StepStatusBadge status={execution.status} />
          {execution.status === 'running' && (
            <Button variant="outline" size="sm" className="gap-2" onClick={cancelExecution}>
              <XCircle size={12} />
              Cancel
            </Button>
          )}
        </div>
      </div>

      {/* Waves */}
      <div className="space-y-4">
        {waves.map((wave, waveIdx) => (
          <div key={waveIdx} className="wave-group space-y-2">
            <div className="text-xs text-muted-foreground font-medium">
              Wave {waveIdx}
              {wave.length > 1 && ` (${wave.length} parallel)`}
            </div>
            {wave.map(({ step, status }) => (
              <div key={step.id} className="border rounded p-3 space-y-2">
                <div className="flex items-center gap-2">
                  <StepStatusBadge status={status} compact />
                  <span className="text-sm font-medium">{step.name}</span>
                  <span className="text-xs text-muted-foreground ml-auto">
                    {step.type}
                  </span>
                </div>
                {/* Streaming output for running steps */}
                {status === 'running' && streamingOutput[step.id]?.length > 0 && (
                  <pre className="text-xs font-mono bg-muted rounded p-2 max-h-24 overflow-auto whitespace-pre-wrap">
                    {streamingOutput[step.id].join('\n')}
                  </pre>
                )}
              </div>
            ))}
          </div>
        ))}
      </div>
    </div>
  )
}

// Helper: group steps by wave number (topological sort)
function groupStepsByWave(
  steps: Array<{ id: string; dependsOn?: string[] }>,
  statuses: Record<string, string>
) {
  const waveMap = new Map<string, number>()

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

  const waveGroups = new Map<number, Array<{ step: typeof steps[0]; status: string }>>()
  for (const [id, wave] of waveMap) {
    if (!waveGroups.has(wave)) waveGroups.set(wave, [])
    const step = steps.find(s => s.id === id)!
    waveGroups.get(wave)!.push({ step, status: statuses[id] ?? 'pending' })
  }

  return [...waveGroups.entries()]
    .sort(([a], [b]) => a - b)
    .map(([, steps]) => steps)
}
```

---

### Hook: `useWorkflow.ts`

**File:** `src/renderer/src/hooks/useWorkflow.ts` (TẠO MỚI)

Implement theo **TDD-FE-14 §5 useWorkflow Hook** (full code có trong spec).

---

### Page: `src/renderer/src/pages/workflows.tsx`

**File:** `src/renderer/src/pages/workflows.tsx` (TẠO MỚI)

```typescript
// BL-WF-01 → BL-WF-03: Workflow page entry point
import { WorkflowTemplateLibrary } from '@/components/workflow/workflow-template-library'

export default function WorkflowsPage() {
  return (
    <div className="workflows-page h-full">
      <WorkflowTemplateLibrary />
    </div>
  )
}
```

---

### Zustand Store Extensions

**File:** `src/renderer/src/store/slices/workflow.ts` (TẠO MỚI)

Theo **TDD-FE-01 §v5.0 Zustand Slices mới**:

```typescript
// workflow slice
type WorkflowSlice = {
  templates: WorkflowDefinition[]
  executions: WorkflowExecution[]
  setTemplates: (templates: WorkflowDefinition[]) => void
  addExecution: (execution: WorkflowExecution) => void
  updateStep: (executionId: string, stepId: string, status: StepStatus) => void
  removeTemplate: (templateId: string) => void
  updateTemplate: (templateId: string, patch: Partial<WorkflowDefinition>) => void
}
```

---

## Files cần tạo

| File | Action | BL |
|------|--------|-----|
| `src/renderer/src/components/workflow/workflow-builder.tsx` | CREATE | BL-WF-01 |
| `src/renderer/src/components/workflow/step-editor.tsx` | CREATE | BL-WF-01 |
| `src/renderer/src/components/workflow/dag-preview.tsx` | CREATE | BL-WF-01 (TDD-FE-14 §3) |
| `src/renderer/src/components/workflow/workflow-template-library.tsx` | CREATE | BL-WF-01, BL-WF-03 |
| `src/renderer/src/components/workflow/workflow-execution-monitor.tsx` | CREATE | BL-WF-02 |
| `src/renderer/src/components/workflow/step-status-badge.tsx` | CREATE | BL-WF-02 |
| `src/renderer/src/hooks/useWorkflow.ts` | CREATE | All |
| `src/renderer/src/store/slices/workflow.ts` | CREATE | Store |
| `src/renderer/src/pages/workflows.tsx` | CREATE | Entry point |

---

## Dependencies

```bash
# React Flow cho DAGPreview (TDD-FE-14 §3):
npm install @xyflow/react

# Verify (có thể đã installed):
grep "@xyflow" package.json
```

---

## Test Coverage

Theo **TDD-FE-14 §6** — target ≥ 25 tests:

```
src/renderer/src/components/workflow/__tests__/
├── WorkflowBuilder.test.tsx
│   ├── adds step with correct defaults
│   ├── removes step and cleans dependsOn references
│   ├── updates step field
│   └── save calls rpc.call('workflow.template.update')
├── DAGPreview.test.tsx
│   ├── linear deps → wave 0 and wave 1 nodes
│   ├── parallel → all in wave 0
│   └── selected step → blue highlight
├── WorkflowExecutionMonitor.test.tsx
│   ├── renders wave groups correctly
│   ├── shows streaming output for running steps
│   └── Cancel button calls workflow.cancel
└── hooks/__tests__/useWorkflow.test.ts
    ├── saveTemplate calls correct RPC (create vs update)
    └── runWorkflow calls workflow.execute
```

---

## Liên quan

- **BL-WF-01**: Template CRUD UI ✅ implemented (WorkflowBuilder + TemplateLibrary)
- **BL-WF-02**: Execution monitor với wave progress ✅ implemented (ExecutionMonitor)
- **BL-WF-03**: Library discovery + share ✅ implemented (TemplateLibrary)
- **TDD-FE-14**: §2 WorkflowBuilder, §3 DAGPreview, §4 ExecutionMonitor, §5 useWorkflow
- **TDD-FE-01**: §v5.0 Workflow slice, @xyflow/react
