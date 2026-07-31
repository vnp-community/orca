# TASK-029: WorkflowOrchestrator

**Phase:** 5 — Workflow Orchestration  
**Solution ref:** [SOL-V5-004](../solutions/SOL-V5-004-workflow-orchestration.md) §5  
**Prerequisite:** TASK-028 (DAGBuilder, types)  
**Status:** ✅ DONE — 2026-07-29

---

## File cần tạo: `src/main/workflow/WorkflowOrchestrator.ts`

Constructor:
```typescript
constructor(
  private readonly pool: IConnectionPool,
  private readonly dagBuilder: DAGBuilder,
  private readonly stepExecutors: StepExecutors,
  private readonly router: ProjectServerRouter
) {}
```

**Public API:**
- `execute(definition, inputs, triggeredBy, projectId?)` → `WorkflowExecution` (persisted, runs async)
- `getExecution(executionId)` → `WorkflowExecution | null`
- `listExecutions(filters)` → `WorkflowExecution[]`
- `cancel(executionId)` → `void` (sets AbortController)
- `resumeRunningExecutions()` → `void` (called at startup)

**Execution flow:**
```
execute() → persist(status=pending) → markRunning → buildWaves → 
forEach wave: Promise.allSettled(executeSteps) → updateWave → 
markCompleted/Failed
```

**Interpolation:** `${inputs.varName}` in step configs.

**AbortController:** Map keyed by executionId. `cancel()` calls `.abort()`.

**DB persistence** — follow SOL-V5-004 §5 patterns:
- `persistExecution()`, `markExecutionRunning()`, `markExecutionCompleted()`, `markExecutionFailed()`
- `updateCurrentWave()`, `persistStepStart()`, `persistStepComplete()`

## Acceptance Criteria

- [x] `WorkflowOrchestrator` class export
- [x] `execute()` persists to DB + runs async
- [x] Wave parallel execution với `Promise.allSettled`
- [x] `cancel()` triggers AbortController
- [x] `resumeRunningExecutions()` queries `status='running'` executions
- [x] Không TypeScript errors
