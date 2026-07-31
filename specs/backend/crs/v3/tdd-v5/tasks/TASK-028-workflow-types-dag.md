# TASK-028: WorkflowTypes + DAGBuilder

**Phase:** 5 — Workflow Orchestration  
**Solution ref:** [SOL-V5-004](../solutions/SOL-V5-004-workflow-orchestration.md) §3, §4  
**Prerequisite:** TASK-020 (ProjectService wired)  
**Status:** ✅ DONE — 2026-07-29

---

## Files cần tạo

### `src/main/workflow/WorkflowTypes.ts`

```typescript
export type WorkflowStepType = 'agent' | 'shell' | 'webhook' | 'notification' | 'condition'
export type WorkflowStatus = 'pending' | 'running' | 'completed' | 'failed' | 'cancelled'

export interface WorkflowStepConfig {
  type: WorkflowStepType
  // agent: { prompt, worktreePath, trustPreset }
  // shell: { script, env }
  // webhook: { url, method, body }
  [key: string]: unknown
}

export interface WorkflowStep {
  id: string
  name: string
  serverSpec: string          // 'project:<projectId>' or 'server:<devServerId>'
  dependsOn?: string[]        // step ids this step waits for
  config: WorkflowStepConfig
  timeout?: number            // ms, default 30min
  continueOnError?: boolean
}

export interface WorkflowDefinition {
  steps: WorkflowStep[]
  inputs?: Record<string, unknown>
}

export interface WorkflowExecution {
  id: string
  definition: WorkflowDefinition
  status: WorkflowStatus
  inputs: Record<string, unknown>
  currentWave: number
  triggeredBy: string
  projectId?: string
  startedAt?: Date
  completedAt?: Date
  errorMessage?: string
  createdAt: Date
}

export interface StepOutput {
  exitCode: number
  stdout?: string
  stderr?: string
  data?: Record<string, unknown>
}

export class WorkflowCycleError extends Error {
  constructor(public readonly cycle: string[]) {
    super(`Workflow cycle detected: ${cycle.join(' → ')}`)
    this.name = 'WorkflowCycleError'
  }
}
```

### `src/main/workflow/DAGBuilder.ts`

Kahn's algorithm topological sort → waves:

```typescript
export class DAGBuilder {
  buildWaves(steps: WorkflowStep[]): WorkflowStep[][] {
    // 1. Build adjacency list + in-degree map
    // 2. Find all nodes with in-degree 0 → first wave
    // 3. Remove those nodes, update in-degrees, repeat
    // 4. If remaining nodes > 0 → cycle detected → throw WorkflowCycleError
    // ...
  }
}
```

**Algorithm:**
1. Map stepId → step
2. For each step, check `dependsOn` — throw `STEP_NOT_FOUND` if ref doesn't exist
3. Compute in-degree (number of deps) per step
4. Queue = steps with in-degree 0
5. While queue not empty: add queue as wave, decrease in-degrees of dependents
6. If total processed < steps.length → cycle → `WorkflowCycleError`

## Acceptance Criteria

- [x] `WorkflowTypes.ts` export all types
- [x] `DAGBuilder.buildWaves()` returns correct wave arrays
- [x] Linear A→B→C = 3 waves
- [x] Parallel A,B,C (no deps) = 1 wave with 3 steps
- [x] Diamond A→B,C→D = 3 waves
- [x] Cycle A→B→A → throws `WorkflowCycleError`
- [x] Missing dep → throws error
- [x] Không TypeScript errors
