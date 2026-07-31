# TASK-V5-20: ExecutionMonitor + Streaming Output

**Order:** 20 | **Prerequisite:** TASK-V5-19 | **Tests:** 10

---

## Files Cần Tạo

### 1. `src/renderer/src/hooks/useWorkflowExecution.ts`

```typescript
export function useWorkflowExecution(executionId: string) {
  const { stepStatuses, streamingOutput } = useAppStore(s => ({
    stepStatuses:    s.stepStatuses[executionId] ?? {},
    streamingOutput: s.streamingOutput,
  }))
  const execution = useAppStore(s => s.executions.find(e => e.id === executionId))

  useEffect(() => {
    if (!window.api?.on) return
    const unsubs = [
      window.api.on('workflow:stepStatus', ({ execId, stepId, status }: any) => {
        if (execId !== executionId) return
        useAppStore.getState().setStepStatus(execId, stepId, status)
      }),
      window.api.on('workflow:stepOutput', ({ execId, line }: any) => {
        if (execId !== executionId) return
        useAppStore.getState().appendStreamLine(execId, line)
      }),
      window.api.on('workflow:complete', ({ execId, status }: any) => {
        if (execId !== executionId) return
        useAppStore.getState().updateExecutionStatus(execId, status)
      }),
    ]
    return () => unsubs.forEach((u: any) => u?.())
  }, [executionId])

  const cancelExecution = useCallback(async () => {
    await callRuntimeRpc('workflow.cancel', { executionId })
    useAppStore.getState().updateExecutionStatus(executionId, 'cancelled')
  }, [executionId])

  return { execution, stepStatuses, streamingOutput, cancelExecution }
}
```

### 2. `src/renderer/src/components/workflow/StepStatusBadge.tsx`

```typescript
import type { StepStatus } from '@shared/workflow-types'
import { CheckCircle2, Loader2, Clock, XCircle, SkipForward } from 'lucide-react'
import { cn } from '../../utils'

const STEP_STATUS: Record<StepStatus, { icon: ReactNode; className: string; label: string }> = {
  pending:   { icon: <Clock      size={14} />, className: 'text-gray-400',  label: 'Pending'   },
  running:   { icon: <Loader2    size={14} className="animate-spin" />, className: 'text-blue-500',  label: 'Running'   },
  completed: { icon: <CheckCircle2 size={14} />, className: 'text-green-500', label: 'Completed' },
  failed:    { icon: <XCircle    size={14} />, className: 'text-red-500',   label: 'Failed'    },
  skipped:   { icon: <SkipForward size={14} />, className: 'text-gray-400', label: 'Skipped'   },
}

export function StepStatusBadge({ status }: { status: StepStatus }) {
  const { icon, className, label } = STEP_STATUS[status]
  return (
    <span className={cn('flex items-center gap-1 text-xs font-medium', className)}>
      {icon} {label}
    </span>
  )
}
```

### 3. `src/renderer/src/components/workflow/ExecutionMonitor.tsx`

```typescript
export function ExecutionMonitor({ executionId }: { executionId: string }) {
  const { execution, stepStatuses, streamingOutput, cancelExecution } = useWorkflowExecution(executionId)

  if (!execution) return <div className="p-4 text-sm text-muted-foreground">Loading execution...</div>

  // Group steps by wave
  const waves = groupStepsByWave(execution.definition.steps, stepStatuses)

  return (
    <div className="execution-monitor p-4 space-y-4" data-testid="execution-monitor">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <div className="font-semibold">{execution.definition.name}</div>
          <div className="text-xs text-muted-foreground">
            Started {new Date(execution.startedAt).toLocaleString()} by {execution.triggeredBy}
          </div>
        </div>
        <div className="flex items-center gap-2">
          <StepStatusBadge status={execution.status as any} />
          {execution.status === 'running' && (
            <Button size="sm" variant="outline" onClick={cancelExecution} data-testid="cancel-btn">
              Cancel
            </Button>
          )}
        </div>
      </div>

      {/* Waves */}
      {waves.map(({ waveIdx, steps }) => (
        <div key={waveIdx} className="wave-group">
          <div className="text-xs text-muted-foreground mb-2">
            Wave {waveIdx} {steps.length > 1 ? `(${steps.length} parallel)` : ''}
          </div>
          {steps.map(({ step, status }) => (
            <div key={step.id} className="step-monitor-row border rounded p-2 mb-1" data-testid={`step-row-${step.id}`}>
              <div className="flex items-center gap-2">
                <StepStatusBadge status={status} />
                <span className="text-sm font-medium">{step.name}</span>
              </div>
              {status === 'running' && (streamingOutput[executionId] ?? []).length > 0 && (
                <pre className="mt-2 text-xs font-mono bg-muted/50 p-2 rounded max-h-32 overflow-y-auto">
                  {(streamingOutput[executionId] ?? []).slice(-50).join('\n')}
                </pre>
              )}
            </div>
          ))}
        </div>
      ))}
    </div>
  )
}

// --- Helpers ---

function groupStepsByWave(steps: WorkflowStep[], statuses: Record<string, StepStatus>) {
  const waveMap = new Map<number, Array<{ step: WorkflowStep; status: StepStatus }>>()
  
  steps.forEach((step, i) => {
    const wave = calculateWave(step, steps)
    if (!waveMap.has(wave)) waveMap.set(wave, [])
    waveMap.get(wave)!.push({ step, status: statuses[step.id] ?? 'pending' })
  })

  return [...waveMap.entries()]
    .sort(([a], [b]) => a - b)
    .map(([waveIdx, steps]) => ({ waveIdx, steps }))
}

function calculateWave(step: WorkflowStep, allSteps: WorkflowStep[]): number {
  if (!step.dependsOn || step.dependsOn.length === 0) return 0
  return Math.max(...step.dependsOn.map(depId => {
    const dep = allSteps.find(s => s.id === depId)
    return dep ? calculateWave(dep, allSteps) + 1 : 0
  }))
}
```

---

## Tests (10 total)

```
__tests__/workflow/ExecutionMonitor.test.tsx   (5 tests)
  renders execution name + started time
  renders wave groups correctly (Wave 0, Wave 1)
  shows streaming output for running steps
  Cancel button calls cancelExecution
  completed step → ✅ StepStatusBadge shows completed

__tests__/workflow/StepStatusBadge.test.tsx    (5 tests)
  pending → Clock icon, gray
  running → spinning Loader2, blue
  completed → CheckCircle, green
  failed → XCircle, red
  skipped → SkipForward, gray
```

## Acceptance Criteria

- [x] `ExecutionMonitor` renders waves correctly
- [x] Steps grouped by `calculateWave()` algorithm
- [x] Streaming output shown for `status='running'` steps
- [x] Cancel button calls `workflow.cancel` RPC
- [x] `StepStatusBadge` 5 variants with correct icon + color
- [x] IPC event subscriptions for step status + output + complete
- [x] 10/10 tests pass
