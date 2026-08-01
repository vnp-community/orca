# TASK-FE-WF-001-E: `ExecutionMonitor` + `useWorkflowExecution` hook (BL-WF-02)

**Domain:** workflow-orchestration  
**Solution Ref:** SOL-FE-WF-001 §ExecutionMonitor + SOL-FE-WF-001B §useWorkflowExecution  
**Bug:** BUG-FE-WF-001  
**Priority:** 🟡 P2  
**Estimated:** 60 phút  
**Status:** ✅ DONE — Already implemented in codebase

---

## Mục tiêu

1. `useWorkflowExecution` — SSE subscription cho real-time step status + streaming output
2. `ExecutionMonitor` — wave-based UI hiển thị execution progress

---

## Files cần tạo

- **MODIFY:** `src/renderer/src/hooks/useWorkflow.ts` — thêm `useWorkflowExecution` export
- **TẠO MỚI:** `src/renderer/src/components/workflow/execution-monitor.tsx`

---

## Bước 1: `useWorkflowExecution(executionId)`

```typescript
// State:
const [stepStatuses, setStepStatuses] = useState<Record<string, string>>({})
const [streamingOutput, setStreamingOutput] = useState<Record<string, string[]>>({})

// SSE subscription via EventSource:
const eventSource = new EventSource(`/api/workflows/${executionId}/stream`)

eventSource.onmessage = (event) => {
  const data = JSON.parse(event.data)
  if (data.type === 'step.status')
    setStepStatuses(prev => ({ ...prev, [data.stepId]: data.status }))
  else if (data.type === 'step.output')
    setStreamingOutput(prev => ({
      ...prev,
      [data.stepId]: [...(prev[data.stepId] ?? []), data.line],
    }))
  else if (data.type === 'execution.complete')
    store.updateExecutionStatus(executionId, data.status)
}

// Cleanup: eventSource.close() on unmount or cancel
```

## Bước 2: `ExecutionMonitor` component

```
Layout:
[Header: "Execution: {name}" | Status badge | Cancel button]
[Sub-header: Started X min ago | Triggered by: {user}]
[Wave 0 (parallel):
  StepMonitorRow for each step in wave 0
    ↳ Running step: show StreamingOutput (last N lines)
]
[Wave 1 (sequential): ...]
```

**`groupStepsByWave(steps, stepStatuses)`** — reuse `buildDAGLayout` logic để group.

**`StepMonitorRow`:** `StepStatusBadge` + step name + duration + server label + streaming output toggle.

---

## Verify

```bash
grep -n "useWorkflowExecution\|ExecutionMonitor\|groupStepsByWave" \
  src/renderer/src/hooks/useWorkflow.ts \
  src/renderer/src/components/workflow/execution-monitor.tsx
```

## Test

```typescript
// ExecutionMonitor.test.tsx
// - renders wave groups correctly from execution.definition.steps
// - shows streaming output for running steps
// - Cancel button calls updateExecutionStatus(..., 'cancelled')
// - completed step shows CheckCircle icon
```

## Depends on
TASK-FE-WF-001-A (slice updateExecutionStatus), TASK-FE-WF-001-C (StepStatusBadge)
