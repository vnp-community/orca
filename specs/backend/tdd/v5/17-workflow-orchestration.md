# TDD-17: Multi-Server Workflow Orchestration

**Document:** TDD-17 (NEW — v5.0)
**Version:** 1.0
**Date:** 2026-07-28
**Domain:** Workflow — DAG orchestration, wave execution, template inheritance, multi-server
**Feature:** F36
**ADR:** ADR-009
**HLD Ref:** C2.15, C3.11c, C4.9
**Source files (to create):**
- `src/main/workflow/WorkflowTypes.ts`
- `src/main/workflow/DAGBuilder.ts`
- `src/main/workflow/WorkflowOrchestrator.ts`
- `src/main/workflow/TemplateResolver.ts`
- `src/main/workflow/StepExecutors.ts`
- `src/main/runtime/rpc/methods/workflow.ts`
- `src/main/db/migrations/0009_workflows.ts`

> **Status: ❌ TODO** — v5.0 proposed; ADR-009: DAG + wave + resumable

---

## 1. Mục tiêu

Chạy automation workflows phức tạp với:
- Steps phụ thuộc vào output của steps khác (DAG)
- Steps song song khi không có dependency (wave execution)
- Steps chạy trên nhiều dev servers khác nhau
- Resumable sau Orca Server restart

---

## 2. Workflow Definition Types

```typescript
// src/main/workflow/WorkflowTypes.ts

export type StepType = 'agent' | 'shell' | 'action' | 'webhook' | 'condition'

export interface WorkflowStep {
  id: string                     // unique within definition
  type: StepType
  name: string
  dependsOn?: string[]           // step IDs this step waits for
  continueOnError?: boolean      // default false — if true, wave continues on step failure
  serverSpec: string             // 'project:<id>' | 'server:<id>' | 'fleet:tag:<tag>'
  providerSpec?: string          // AI provider account ID or 'resolve:server'
  config: StepConfig             // type-specific config
  timeout?: number               // ms, default 30min, max 2h
}

export type StepConfig =
  | { type: 'agent';    prompt: string; worktreePath: string; trustPreset?: string }
  | { type: 'shell';    command: string; args?: string[]; cwd?: string }
  | { type: 'action';   actionId: string; params?: Record<string, unknown> }
  | { type: 'webhook';  url: string; method: 'GET'|'POST'; body?: unknown }
  | { type: 'condition'; expression: string; trueBranch: string; falseBranch: string }

export interface WorkflowDefinition {
  id: string
  name: string
  version: number
  templateId?: string            // parent template for inheritance
  description?: string
  inputs?: Record<string, { type: string; required: boolean; default?: unknown }>
  steps: WorkflowStep[]
}

export type WorkflowStatus = 'pending' | 'running' | 'paused' | 'completed' | 'failed' | 'cancelled'
export type StepStatus = 'pending' | 'running' | 'completed' | 'failed' | 'skipped' | 'timeout'

export interface StepOutput {
  exitCode?: number
  stdout?: string
  stderr?: string
  data?: unknown               // for action/webhook results
}
```

---

## 3. DAGBuilder

```typescript
// src/main/workflow/DAGBuilder.ts

export interface DAGNode {
  step: WorkflowStep
  wave: number                 // parallel execution group (0 = first)
  predecessors: string[]       // step IDs that must complete before this
  successors: string[]         // step IDs that depend on this
}

export class DAGBuilder {
  /**
   * Build DAG from step list using Kahn's topological sort algorithm.
   * @throws WorkflowCycleError if cycle detected
   */
  build(steps: WorkflowStep[]): DAGNode[][] {
    const stepMap = new Map(steps.map(s => [s.id, s]))
    const inDegree = new Map<string, number>()
    const adjacency = new Map<string, string[]>()

    // Initialize
    for (const step of steps) {
      inDegree.set(step.id, 0)
      adjacency.set(step.id, [])
    }

    // Build adjacency list and in-degrees
    for (const step of steps) {
      for (const dep of step.dependsOn ?? []) {
        if (!stepMap.has(dep)) throw new Error(`STEP_NOT_FOUND: ${dep}`)
        adjacency.get(dep)!.push(step.id)
        inDegree.set(step.id, (inDegree.get(step.id) ?? 0) + 1)
      }
    }

    // Kahn's algorithm — produces topological order with wave grouping
    const waves: DAGNode[][] = []
    let queue = [...inDegree.entries()]
      .filter(([, deg]) => deg === 0)
      .map(([id]) => id)

    const nodes = new Map<string, DAGNode>()
    for (const step of steps) {
      nodes.set(step.id, {
        step,
        wave: 0,
        predecessors: step.dependsOn ?? [],
        successors: adjacency.get(step.id) ?? [],
      })
    }

    while (queue.length > 0) {
      // All nodes in queue have no remaining dependencies → same wave
      const currentWaveIds = [...queue]
      const currentWave: DAGNode[] = currentWaveIds.map(id => nodes.get(id)!)
      waves.push(currentWave)

      // Set wave number
      for (const id of currentWaveIds) {
        nodes.get(id)!.wave = waves.length - 1
      }

      const nextQueue: string[] = []
      for (const id of currentWaveIds) {
        for (const successor of adjacency.get(id) ?? []) {
          const newDegree = (inDegree.get(successor) ?? 0) - 1
          inDegree.set(successor, newDegree)
          if (newDegree === 0) nextQueue.push(successor)
        }
      }
      queue = nextQueue
    }

    // Cycle detection: if any node still has in-degree > 0
    const remaining = [...inDegree.entries()].filter(([, d]) => d > 0)
    if (remaining.length > 0) {
      throw new WorkflowCycleError(remaining.map(([id]) => id))
    }

    return waves
  }
}

export class WorkflowCycleError extends Error {
  constructor(public readonly cycleNodes: string[]) {
    super(`Workflow cycle detected involving steps: ${cycleNodes.join(', ')}`)
  }
}
```

---

## 4. WorkflowOrchestrator

```typescript
// src/main/workflow/WorkflowOrchestrator.ts

export class WorkflowOrchestrator {
  private runningExecutions = new Map<string, AbortController>()

  constructor(
    private readonly pool: IConnectionPool,
    private readonly dagBuilder: DAGBuilder,
    private readonly stepExecutors: StepExecutors,
    private readonly serverResolver: ServerResolver
  ) {}

  /** Start a new workflow execution */
  async start(params: {
    definitionOrTemplateId: string | WorkflowDefinition
    inputs?: Record<string, unknown>
    triggeredBy: string
    projectId?: string
  }): Promise<string> {  // returns executionId
    const definition = typeof params.definitionOrTemplateId === 'string'
      ? await this.loadTemplate(params.definitionOrTemplateId)
      : params.definitionOrTemplateId

    const executionId = randomUUID()
    await this.persistExecution({
      id: executionId,
      definitionSnapshot: JSON.stringify(definition),
      status: 'pending',
      inputsJson: JSON.stringify(params.inputs ?? {}),
      currentWave: 0,
      triggeredBy: params.triggeredBy,
      projectId: params.projectId,
    })

    // Run async
    this.runExecution(executionId, definition, params.inputs ?? {}).catch(err => {
      this.markExecutionFailed(executionId, err.message)
    })

    return executionId
  }

  private async runExecution(
    executionId: string,
    definition: WorkflowDefinition,
    inputs: Record<string, unknown>
  ): Promise<void> {
    const waves = this.dagBuilder.build(definition.steps)
    const outputs = new Map<string, StepOutput>()  // stepId → output
    const abortController = new AbortController()
    this.runningExecutions.set(executionId, abortController)

    await this.markExecutionRunning(executionId)

    try {
      for (let waveIndex = 0; waveIndex < waves.length; waveIndex++) {
        if (abortController.signal.aborted) break
        await this.updateCurrentWave(executionId, waveIndex)

        const wave = waves[waveIndex]
        // Interpolate variables before execution
        const interpolatedSteps = wave.map(node =>
          this.interpolateStep(node.step, inputs, outputs)
        )

        // Execute all steps in wave concurrently
        const results = await Promise.allSettled(
          interpolatedSteps.map(step => this.executeStep(executionId, step, abortController.signal))
        )

        // Collect outputs and check failures
        let anyHardFailure = false
        for (let i = 0; i < results.length; i++) {
          const result = results[i]
          const step = wave[i].step
          if (result.status === 'fulfilled') {
            outputs.set(step.id, result.value)
          } else {
            if (!step.continueOnError) anyHardFailure = true
          }
        }

        if (anyHardFailure) {
          await this.markExecutionFailed(executionId, 'Step failed without continueOnError')
          return
        }
      }

      await this.markExecutionCompleted(executionId)
    } finally {
      this.runningExecutions.delete(executionId)
    }
  }

  /** Interpolate {{inputs.*}} and {{outputs.<stepId>.*}} in step config */
  private interpolateStep(
    step: WorkflowStep,
    inputs: Record<string, unknown>,
    outputs: Map<string, StepOutput>
  ): WorkflowStep {
    const configStr = JSON.stringify(step.config)
    const interpolated = configStr
      .replace(/\{\{inputs\.(\w+)\}\}/g, (_, k) => String(inputs[k] ?? ''))
      .replace(/\{\{outputs\.(\w+)\.(\w+)\}\}/g, (_, stepId, field) => {
        const output = outputs.get(stepId) as any
        return String(output?.[field] ?? '')
      })
    return { ...step, config: JSON.parse(interpolated) }
  }

  /** Resume executions on startup (after Orca restart) */
  async resumeRunningExecutions(): Promise<void> {
    const interrupted = await this.pool.query<{ id: string; definitionSnapshot: string; currentWave: number }>(
      `SELECT id, definition_snapshot as definitionSnapshot, current_wave as currentWave
       FROM orca_workflow_executions WHERE status = 'running'`
    )
    for (const exec of interrupted) {
      const definition = JSON.parse(exec.definitionSnapshot) as WorkflowDefinition
      // Skip waves already completed (currentWave already stores last completed wave)
      this.runExecution(exec.id, definition, {}).catch(() => {})
    }
  }

  cancel(executionId: string): void {
    this.runningExecutions.get(executionId)?.abort()
  }
}
```

---

## 5. TemplateResolver — Inheritance

```typescript
// src/main/workflow/TemplateResolver.ts

const MAX_INHERIT_DEPTH = 5

export class TemplateResolver {
  constructor(private readonly pool: IConnectionPool) {}

  async resolve(templateId: string, overrides?: Partial<WorkflowDefinition>): Promise<WorkflowDefinition> {
    const chain = await this.loadInheritanceChain(templateId, 0)
    // Merge from root to leaf (leaf overrides)
    let merged = chain[0]!
    for (const template of chain.slice(1)) {
      merged = this.mergeDefinitions(merged, template)
    }
    if (overrides) {
      merged = this.mergeDefinitions(merged, overrides as WorkflowDefinition)
    }
    return merged
  }

  private async loadInheritanceChain(
    templateId: string, depth: number
  ): Promise<WorkflowDefinition[]> {
    if (depth >= MAX_INHERIT_DEPTH) throw new Error('TEMPLATE_INHERIT_MAX_DEPTH')
    const row = await this.pool.queryOne<{ definitionJson: string; parentTemplateId?: string }>(
      `SELECT definition_json as definitionJson, parent_template_id as parentTemplateId
       FROM orca_workflow_templates WHERE id = ?`, [templateId]
    )
    if (!row) throw new Error(`TEMPLATE_NOT_FOUND: ${templateId}`)
    const def = JSON.parse(row.definitionJson) as WorkflowDefinition
    if (!row.parentTemplateId) return [def]
    const parentChain = await this.loadInheritanceChain(row.parentTemplateId, depth + 1)
    return [...parentChain, def]
  }

  private mergeDefinitions(base: WorkflowDefinition, override: WorkflowDefinition): WorkflowDefinition {
    // Steps: override by step.id
    const stepMap = new Map(base.steps.map(s => [s.id, s]))
    for (const step of override.steps) stepMap.set(step.id, step)
    return { ...base, ...override, steps: [...stepMap.values()] }
  }
}
```

---

## 6. RPC Methods

```typescript
// namespace: 'workflow'

'workflow.template.create'   // (admin/team-lead) → templateId
'workflow.template.get'      // (member) → WorkflowDefinition
'workflow.template.list'     // (member) → template list filtered by scope
'workflow.template.update'   // (owner) → void
'workflow.template.delete'   // (owner/admin) → void
'workflow.template.share'    // (owner) → { shareToken }

'workflow.execute'           // (member with execute) → executionId
'workflow.cancel'            // (triggeredBy or admin) → void
'workflow.getExecution'      // (member) → WorkflowExecution + step statuses
'workflow.listExecutions'    // (member) → WorkflowExecution[]
'workflow.resumeAll'         // (admin) → void — resume interrupted executions
```

---

## 7. Error Handling

| Scenario | Error code |
|---------|-----------|
| Cycle in DAG | `WORKFLOW_CYCLE` — 400 with cycleNodes list |
| Template not found | `TEMPLATE_NOT_FOUND` — 404 |
| Inherit depth > 5 | `TEMPLATE_INHERIT_MAX_DEPTH` — 400 |
| Step server unreachable | `STEP_SERVER_UNREACHABLE` — propagate or continueOnError |
| Step timeout | `STEP_TIMEOUT` — mark step as timeout |
| No AI provider | `NO_AI_PROVIDER_CONFIGURED` — fail step |
| Execution cancel | `EXECUTION_CANCELLED` — mark all pending steps as skipped |

---

## 8. Test Coverage

```
src/main/workflow/__tests__/
├── DAGBuilder.test.ts
│   ├── linear dependency → sequential waves
│   ├── parallel (no deps) → single wave
│   ├── diamond dependency → 3 waves
│   ├── cycle detection → WorkflowCycleError
│   └── missing dependsOn step → STEP_NOT_FOUND
├── WorkflowOrchestrator.test.ts
│   ├── execute: 2-wave workflow, wave 0 parallel, wave 1 after
│   ├── execute: step failure without continueOnError → abort
│   ├── execute: step failure with continueOnError → next wave continues
│   ├── cancel: abort in-progress wave
│   ├── resume: load 'running' executions from DB
│   └── interpolate: {{inputs.branch}} and {{outputs.step1.stdout}}
├── TemplateResolver.test.ts
│   ├── no parent → base definition returned
│   ├── 2-level inherit → step override by id
│   ├── max depth > 5 → TEMPLATE_INHERIT_MAX_DEPTH
│   └── overrides merge on top
└── workflow-rpc.test.ts
    ├── workflow.execute → executionId returned
    └── workflow.getExecution → correct status
```

**Target:** ≥ 45 tests; DAGBuilder fully tested for correctness
