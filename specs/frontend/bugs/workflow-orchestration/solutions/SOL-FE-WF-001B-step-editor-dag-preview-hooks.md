# SOL-FE-WF-001B: StepEditor, DAGPreview, StepStatusBadge, useWorkflowExecution hook

## Bug Reference
- **Bug:** BUG-FE-WF-001 (Supplement — thiếu component code chi tiết)
- **Depends on:** SOL-FE-WF-001 (WorkflowBuilder, TemplateLibrary, ExecutionMonitor)
- **TDD Reference:** TDD-FE-14 §2–§5 (đầy đủ implementation)

---

## Lý do bổ sung

SOL-FE-WF-001 đã có WorkflowBuilder, TemplateLibrary, ExecutionMonitor nhưng thiếu code cho:
- `StepEditor.tsx` — edit step config (type, server, prompt, dependsOn, timeout)
- `DAGPreview.tsx` — React Flow visualization (copy từ TDD-FE-14 §3)
- `StepStatusBadge.tsx` — status icon component
- `useWorkflowExecution()` hook — SSE streaming step output

---

### Component 1: `step-editor.tsx`

**File:** `src/renderer/src/components/workflow/step-editor.tsx` (TẠO MỚI)

```typescript
// src/renderer/src/components/workflow/step-editor.tsx
// Edit form cho một workflow step (type, server, prompt/command, dependsOn)

import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Checkbox } from '@/components/ui/checkbox'
import { Trash2 } from 'lucide-react'

type StepType = 'agent' | 'shell' | 'notify'

type WorkflowStep = {
  id: string
  type: StepType
  name: string
  serverSpec: string
  config: AgentStepConfig | ShellStepConfig | NotifyStepConfig
  dependsOn: string[]
  continueOnError?: boolean
  timeoutMinutes?: number
}

type AgentStepConfig  = { type: 'agent';  prompt: string; worktreePath: string }
type ShellStepConfig  = { type: 'shell';  command: string; workdir: string }
type NotifyStepConfig = { type: 'notify'; channel: string; message: string }

interface StepEditorProps {
  step: WorkflowStep
  allSteps: WorkflowStep[]
  onUpdate: (patch: Partial<WorkflowStep>) => void
  onDelete: () => void
}

export function StepEditor({ step, allSteps, onUpdate, onDelete }: StepEditorProps) {
  // Other steps as potential dependencies (exclude self)
  const potentialDeps = allSteps.filter(s => s.id !== step.id)

  const updateConfig = (patch: Partial<typeof step.config>) => {
    onUpdate({ config: { ...step.config, ...patch } as WorkflowStep['config'] })
  }

  const toggleDep = (depId: string, checked: boolean) => {
    const deps = checked
      ? [...step.dependsOn, depId]
      : step.dependsOn.filter(d => d !== depId)
    onUpdate({ dependsOn: deps })
  }

  return (
    <div className="step-editor p-4 space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-semibold">Edit Step</h3>
        <Button
          variant="ghost"
          size="icon"
          className="h-7 w-7 text-destructive"
          onClick={onDelete}
          title="Delete step"
        >
          <Trash2 size={14} />
        </Button>
      </div>

      {/* Step Name */}
      <div className="space-y-1">
        <Label className="text-xs">Name</Label>
        <Input
          value={step.name}
          onChange={e => onUpdate({ name: e.target.value })}
          placeholder="Step name..."
          className="text-sm"
        />
      </div>

      {/* Step Type */}
      <div className="space-y-1">
        <Label className="text-xs">Type</Label>
        <Select
          value={step.type}
          onValueChange={type => {
            const defaultConfigs: Record<StepType, WorkflowStep['config']> = {
              agent:  { type: 'agent',  prompt: '', worktreePath: '.' },
              shell:  { type: 'shell',  command: '', workdir: '.' },
              notify: { type: 'notify', channel: 'slack', message: '' },
            }
            onUpdate({ type: type as StepType, config: defaultConfigs[type as StepType] })
          }}
        >
          <SelectTrigger className="text-sm">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="agent">Agent (AI)</SelectItem>
            <SelectItem value="shell">Shell command</SelectItem>
            <SelectItem value="notify">Notification</SelectItem>
          </SelectContent>
        </Select>
      </div>

      {/* Server spec */}
      <div className="space-y-1">
        <Label className="text-xs">Server</Label>
        <Input
          value={step.serverSpec}
          onChange={e => onUpdate({ serverSpec: e.target.value })}
          placeholder="project:current or server-id"
          className="text-sm font-mono"
        />
        <p className="text-xs text-muted-foreground">
          Use "project:current" or a specific SSH server ID
        </p>
      </div>

      {/* Type-specific config */}
      {step.type === 'agent' && (
        <>
          <div className="space-y-1">
            <Label className="text-xs">Prompt</Label>
            <Textarea
              value={(step.config as AgentStepConfig).prompt}
              onChange={e => updateConfig({ prompt: e.target.value })}
              placeholder="Describe what the agent should do..."
              rows={4}
              className="text-sm resize-none"
            />
          </div>
          <div className="space-y-1">
            <Label className="text-xs">Worktree Path</Label>
            <Input
              value={(step.config as AgentStepConfig).worktreePath}
              onChange={e => updateConfig({ worktreePath: e.target.value })}
              placeholder="."
              className="text-sm font-mono"
            />
          </div>
        </>
      )}

      {step.type === 'shell' && (
        <>
          <div className="space-y-1">
            <Label className="text-xs">Command</Label>
            <Textarea
              value={(step.config as ShellStepConfig).command}
              onChange={e => updateConfig({ command: e.target.value })}
              placeholder="npm test"
              rows={3}
              className="text-sm font-mono resize-none"
            />
          </div>
          <div className="space-y-1">
            <Label className="text-xs">Working Directory</Label>
            <Input
              value={(step.config as ShellStepConfig).workdir}
              onChange={e => updateConfig({ workdir: e.target.value })}
              placeholder="."
              className="text-sm font-mono"
            />
          </div>
        </>
      )}

      {step.type === 'notify' && (
        <>
          <div className="space-y-1">
            <Label className="text-xs">Channel</Label>
            <Input
              value={(step.config as NotifyStepConfig).channel}
              onChange={e => updateConfig({ channel: e.target.value })}
              placeholder="slack or teams or email"
              className="text-sm"
            />
          </div>
          <div className="space-y-1">
            <Label className="text-xs">Message</Label>
            <Textarea
              value={(step.config as NotifyStepConfig).message}
              onChange={e => updateConfig({ message: e.target.value })}
              placeholder="Deployment completed!"
              rows={2}
              className="text-sm resize-none"
            />
          </div>
        </>
      )}

      {/* Depends On */}
      {potentialDeps.length > 0 && (
        <div className="space-y-1">
          <Label className="text-xs">Depends On</Label>
          <div className="space-y-1 max-h-32 overflow-y-auto">
            {potentialDeps.map(dep => (
              <label key={dep.id} className="flex items-center gap-2 text-xs cursor-pointer">
                <Checkbox
                  checked={step.dependsOn.includes(dep.id)}
                  onCheckedChange={checked => toggleDep(dep.id, !!checked)}
                />
                {dep.name}
              </label>
            ))}
          </div>
        </div>
      )}

      {/* Options */}
      <div className="space-y-2">
        <label className="flex items-center gap-2 text-xs cursor-pointer">
          <Checkbox
            checked={step.continueOnError ?? false}
            onCheckedChange={checked => onUpdate({ continueOnError: !!checked })}
          />
          Continue workflow on error
        </label>

        <div className="space-y-1">
          <Label className="text-xs">Timeout (minutes)</Label>
          <Input
            type="number"
            value={step.timeoutMinutes ?? 30}
            onChange={e => onUpdate({ timeoutMinutes: parseInt(e.target.value) || 30 })}
            min={1}
            max={480}
            className="text-sm w-24"
          />
        </div>
      </div>
    </div>
  )
}
```

---

### Component 2: `dag-preview.tsx`

**File:** `src/renderer/src/components/workflow/dag-preview.tsx` (TẠO MỚI)

Đây là **copy trực tiếp từ TDD-FE-14 §3** với đầy đủ implementation:

```typescript
// src/renderer/src/components/workflow/dag-preview.tsx
// Theo TDD-FE-14 §3 — sử dụng @xyflow/react (React Flow v12)

import { useMemo } from 'react'
import { ReactFlow, Node, Edge, Background, Controls } from '@xyflow/react'
import '@xyflow/react/dist/style.css'

interface WorkflowStep {
  id: string
  type: string
  name: string
  dependsOn?: string[]
}

interface DAGPreviewProps {
  steps: WorkflowStep[]
  selectedStepId: string | null
}

export function DAGPreview({ steps, selectedStepId }: DAGPreviewProps) {
  const { nodes, edges } = useMemo(() => buildDAGLayout(steps), [steps])

  return (
    <div className="dag-preview h-full min-h-[200px] border rounded bg-muted/30">
      {steps.length === 0 ? (
        <div className="flex items-center justify-center h-full text-xs text-muted-foreground">
          Add steps to see DAG preview
        </div>
      ) : (
        <ReactFlow
          nodes={nodes.map(n => ({
            ...n,
            selected: n.id === selectedStepId,
            style: {
              background: n.id === selectedStepId ? '#dbeafe' : '#f8fafc',
              border: n.id === selectedStepId ? '2px solid #3b82f6' : '1px solid #e2e8f0',
              borderRadius: 8,
              fontSize: 11,
              padding: '4px 8px',
              minWidth: 80,
            }
          }))}
          edges={edges}
          fitView
          fitViewOptions={{ padding: 0.2 }}
          nodesDraggable={false}
          nodesConnectable={false}
          elementsSelectable={false}
          proOptions={{ hideAttribution: true }}
        >
          <Background color="#94a3b8" gap={20} size={0.5} />
          <Controls showInteractive={false} />
        </ReactFlow>
      )}
    </div>
  )
}

function buildDAGLayout(steps: WorkflowStep[]): { nodes: Node[]; edges: Edge[] } {
  const nodes: Node[] = []
  const edges: Edge[] = []
  const waveMap = new Map<string, number>()

  // Topological sort → assign wave numbers
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

  // Group by wave → calculate positions
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
        position: { x: wave * 180, y: idx * 70 },
        data: { label: `${step.name}\n(${step.type})` },
        type: 'default',
      })
    })
  }

  // Build edges from dependsOn
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

### Component 3: `step-status-badge.tsx`

**File:** `src/renderer/src/components/workflow/step-status-badge.tsx` (TẠO MỚI)

```typescript
// Status badge cho workflow execution steps
import { Badge } from '@/components/ui/badge'
import { CheckCircle, XCircle, Loader2, Clock, AlertTriangle } from 'lucide-react'

type StepStatus = 'pending' | 'running' | 'completed' | 'failed' | 'skipped'

interface StepStatusBadgeProps {
  status: StepStatus | string
  compact?: boolean  // minimal icon-only version
}

const STATUS_CONFIG: Record<string, {
  icon: React.ElementType
  label: string
  className: string
}> = {
  pending:   { icon: Clock,         label: 'Pending',   className: 'text-muted-foreground' },
  running:   { icon: Loader2,       label: 'Running',   className: 'text-blue-500 animate-spin' },
  completed: { icon: CheckCircle,   label: 'Done',      className: 'text-green-500' },
  failed:    { icon: XCircle,       label: 'Failed',    className: 'text-destructive' },
  skipped:   { icon: AlertTriangle, label: 'Skipped',   className: 'text-yellow-500' },
}

export function StepStatusBadge({ status, compact = false }: StepStatusBadgeProps) {
  const config = STATUS_CONFIG[status] ?? STATUS_CONFIG.pending
  const Icon = config.icon

  if (compact) {
    return <Icon size={14} className={config.className} />
  }

  return (
    <Badge
      variant="outline"
      className={`gap-1 text-xs ${config.className}`}
    >
      <Icon size={10} className={status === 'running' ? 'animate-spin' : ''} />
      {config.label}
    </Badge>
  )
}
```

---

### Hook: `useWorkflowExecution.ts`

**File:** `src/renderer/src/hooks/useWorkflow.ts` — thêm `useWorkflowExecution` export (MODIFY)

```typescript
// Thêm vào src/renderer/src/hooks/useWorkflow.ts:

// useWorkflowExecution — real-time SSE monitoring
export function useWorkflowExecution(executionId: string) {
  const { executions } = useAppStore(useShallow(s => ({ executions: s.executions as WorkflowExecution[] })))
  const execution = executions.find(e => e.id === executionId)

  const [stepStatuses, setStepStatuses] = useState<Record<string, string>>({})
  const [streamingOutput, setStreamingOutput] = useState<Record<string, string[]>>({})

  useEffect(() => {
    if (!executionId) return

    // Subscribe to SSE stream: workflow.streamStepOutput (TDD-FE-14 §E.7)
    let cancelled = false

    async function subscribeSSE() {
      try {
        // Use rpc.call with AsyncIterable pattern (per HLD E.7):
        // 'workflows.streamStepOutput' returns SSE events
        const eventSource = new EventSource(`/api/workflows/${executionId}/stream`)

        eventSource.onmessage = (event) => {
          if (cancelled) { eventSource.close(); return }
          try {
            const data = JSON.parse(event.data) as WorkflowStreamEvent
            if (data.type === 'step.status') {
              setStepStatuses(prev => ({ ...prev, [data.stepId]: data.status }))
            } else if (data.type === 'step.output') {
              setStreamingOutput(prev => ({
                ...prev,
                [data.stepId]: [...(prev[data.stepId] ?? []), data.line],
              }))
            } else if (data.type === 'execution.complete') {
              eventSource.close()
              // Update store
              useAppStore.getState().updateExecutionStatus?.(executionId, data.status)
            }
          } catch {}
        }

        eventSource.onerror = () => {
          if (!cancelled) eventSource.close()
        }

        return () => {
          cancelled = true
          eventSource.close()
        }
      } catch (err) {
        console.error('Failed to subscribe to execution stream:', err)
      }
    }

    const cleanup = subscribeSSE()
    return () => { cancelled = true; cleanup?.then(fn => fn?.()) }
  }, [executionId])

  // Sync stepStatuses from store execution data
  useEffect(() => {
    if (!execution?.stepStatuses) return
    setStepStatuses(execution.stepStatuses)
  }, [execution?.stepStatuses])

  return { execution, stepStatuses, streamingOutput }
}

// Types needed:
type WorkflowStreamEvent =
  | { type: 'step.status'; stepId: string; status: string }
  | { type: 'step.output'; stepId: string; line: string }
  | { type: 'execution.complete'; status: 'completed' | 'failed' }

type WorkflowExecution = {
  id: string
  name?: string
  status: string
  startedAt: number
  triggeredBy: string
  definition?: { steps: any[] }
  stepStatuses?: Record<string, string>
}
```

---

### Zustand Workflow Slice chi tiết

**File:** `src/renderer/src/store/slices/workflow.ts` (TẠO MỚI)

Bổ sung actions còn thiếu trong SOL-FE-WF-001:

```typescript
// src/renderer/src/store/slices/workflow.ts
// Theo TDD-FE-01 §E.1 — Workflow Slice (HLD C4.9)

import type { StateCreator } from 'zustand'
import type { AppState } from '../index'

export type WorkflowDefinition = {
  id: string
  name: string
  scope: 'personal' | 'team' | 'public'
  steps: WorkflowStep[]
  templateId?: string       // parent template (inheritance)
  stepsCount: number
  lastModified: number
  ownerId: string
}

export type WorkflowStep = {
  id: string
  type: 'agent' | 'shell' | 'notify'
  name: string
  serverSpec: string
  config: Record<string, unknown>
  dependsOn: string[]
  continueOnError?: boolean
  timeoutMinutes?: number
}

export type WorkflowExecution = {
  id: string
  templateId: string
  name?: string
  status: 'running' | 'completed' | 'failed' | 'cancelled'
  startedAt: number
  triggeredBy: string
  definition?: { steps: WorkflowStep[] }
  stepStatuses?: Record<string, string>
}

type WorkflowSlice = {
  templates: WorkflowDefinition[]
  executions: WorkflowExecution[]

  setTemplates: (templates: WorkflowDefinition[]) => void
  addTemplate: (template: WorkflowDefinition) => void
  updateTemplate: (templateId: string, patch: Partial<WorkflowDefinition>) => void
  removeTemplate: (templateId: string) => void

  addExecution: (execution: WorkflowExecution) => void
  updateExecutionStatus: (executionId: string, status: WorkflowExecution['status']) => void
  updateStep: (executionId: string, stepId: string, status: string) => void
}

export const createWorkflowSlice: StateCreator<AppState, [], [], WorkflowSlice> = (set) => ({
  templates: [],
  executions: [],

  setTemplates: (templates) => set({ templates }),

  addTemplate: (template) =>
    set(s => ({ templates: [template, ...s.templates] })),

  updateTemplate: (templateId, patch) =>
    set(s => ({
      templates: s.templates.map(t =>
        t.id === templateId ? { ...t, ...patch, lastModified: Date.now() } : t
      ),
    })),

  removeTemplate: (templateId) =>
    set(s => ({ templates: s.templates.filter(t => t.id !== templateId) })),

  addExecution: (execution) =>
    set(s => ({ executions: [execution, ...s.executions] })),

  updateExecutionStatus: (executionId, status) =>
    set(s => ({
      executions: s.executions.map(e =>
        e.id === executionId ? { ...e, status } : e
      ),
    })),

  updateStep: (executionId, stepId, status) =>
    set(s => ({
      executions: s.executions.map(e =>
        e.id === executionId
          ? { ...e, stepStatuses: { ...e.stepStatuses, [stepId]: status } }
          : e
      ),
    })),
})
```

---

## Files cần tạo (BỔ SUNG vào SOL-FE-WF-001)

| File | Action | Ghi chú |
|------|--------|---------|
| `src/renderer/src/components/workflow/step-editor.tsx` | CREATE | ← **THIẾU** trong SOL-001 |
| `src/renderer/src/components/workflow/dag-preview.tsx` | CREATE | ← **THIẾU** code chi tiết |
| `src/renderer/src/components/workflow/step-status-badge.tsx` | CREATE | ← **THIẾU** |
| `src/renderer/src/store/slices/workflow.ts` | CREATE | ← **THIẾU** full implementation |
| `src/renderer/src/hooks/useWorkflow.ts` | MODIFY | Thêm `useWorkflowExecution` export |

---

## Liên quan

- **BL-WF-01**: Template CRUD UI — StepEditor ✅ added
- **BL-WF-02**: Execution Monitor — `useWorkflowExecution` + `StepStatusBadge` ✅ added
- **BL-WF-01**: DAGPreview ✅ complete code
- **TDD-FE-14**: §3 DAGPreview, §5 useWorkflow, §E.7 SSE streaming
