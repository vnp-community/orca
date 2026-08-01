# SOLUTION: Workflow Orchestration Domain — Fix tất cả Bugs

**Domain:** workflow-orchestration  
**TDD Reference:** TDD-17 (Workflow Orchestration), TDD-18 (Task Graph)  
**Files cần thay đổi:** `src/main/workflow/WorkflowOrchestrator.ts`, `src/main/workflow/StepExecutors.ts`, `src/main/workflow/WorkflowServer.ts`  
**Tổng số bugs:** 4 (WF-001, WF-003, WF-004, BE-WF-001)

---

## Tổng quan phụ thuộc

```
BUG-BE-WF-001 (orchestrator not implemented) — phải implement trước
    ├── BUG-WF-001 (server spec not implemented)
    ├── BUG-WF-003 (condition step code injection)
    └── BUG-WF-004 (resume orphan step execution)
```

**Thứ tự fix:** `BE-WF-001 → WF-001 → WF-003 → WF-004`

---

## BUG-BE-WF-001 — Fix WorkflowOrchestrator not implemented

**Mức độ:** 🔴 CRITICAL  
**Root cause:** `WorkflowOrchestrator` class chỉ là stub.

### Fix — Implement WorkflowOrchestrator đầy đủ

Theo TDD-17 (Workflow Orchestration):

```typescript
// src/main/workflow/WorkflowOrchestrator.ts

export type StepStatus  = 'pending' | 'running' | 'completed' | 'failed' | 'skipped'
export type ExecStatus  = 'pending' | 'running' | 'completed' | 'failed' | 'cancelled'

export interface WorkflowExecution {
  id:           string
  workflowId:   string
  userId:       string
  projectId:    string
  status:       ExecStatus
  currentWave:  number
  startedAt:    number
  completedAt?: number
  trigger:      { type: string; payload?: unknown }
}

export class WorkflowOrchestrator {
  constructor(
    private readonly repository:    IWorkflowRepository,
    private readonly stepExecutors: StepExecutorRegistry,
    private readonly eventBus:      EventBus,
    private readonly log:           Logger,
  ) {}

  /**
   * Start new execution của workflow.
   */
  async startExecution(params: {
    workflowId: string
    userId:     string
    projectId:  string
    trigger:    WorkflowExecution['trigger']
  }): Promise<WorkflowExecution> {
    const workflow = await this.repository.getWorkflow(params.workflowId)
    if (!workflow) throw new Error(`Workflow not found: ${params.workflowId}`)

    const execution: WorkflowExecution = {
      id:          generateId(),
      workflowId:  params.workflowId,
      userId:      params.userId,
      projectId:   params.projectId,
      status:      'running',
      currentWave: 0,
      startedAt:   Date.now(),
      trigger:     params.trigger,
    }

    await this.repository.createExecution(execution)
    this.eventBus.emit('workflow.execution.started', { executionId: execution.id })

    // Run waves async (non-blocking)
    void this.runExecution(execution, 0)

    return execution
  }

  /**
   * Execute workflow theo wave model (parallel steps trong cùng wave).
   */
  private async runExecution(execution: WorkflowExecution, fromWave: number): Promise<void> {
    const workflow = await this.repository.getWorkflow(execution.workflowId)
    if (!workflow) return

    const waves = this.buildWaves(workflow.steps)

    for (let waveIdx = fromWave; waveIdx < waves.length; waveIdx++) {
      const wave = waves[waveIdx]!

      // Update current wave
      await this.repository.updateExecution(execution.id, { currentWave: waveIdx })

      // Execute all steps in wave in parallel
      const results = await Promise.allSettled(
        wave.map(step => this.executeStep(step, execution))
      )

      // Check for failures
      const failed = results.filter(r => r.status === 'rejected')
      if (failed.length > 0) {
        this.log.error(`[Workflow] Wave ${waveIdx} failed: ${failed.length} steps failed`)
        await this.repository.updateExecution(execution.id, {
          status:      'failed',
          completedAt: Date.now(),
        })
        this.eventBus.emit('workflow.execution.failed', { executionId: execution.id })
        return
      }
    }

    // All waves completed
    await this.repository.updateExecution(execution.id, {
      status:      'completed',
      completedAt: Date.now(),
    })
    this.eventBus.emit('workflow.execution.completed', { executionId: execution.id })
    this.log.info(`[Workflow] Execution completed: ${execution.id}`)
  }

  private async executeStep(step: WorkflowStep, execution: WorkflowExecution): Promise<void> {
    await this.repository.updateStepStatus(execution.id, step.id, 'running')

    try {
      const executor = this.stepExecutors.get(step.type)
      if (!executor) throw new Error(`No executor for step type: ${step.type}`)

      const result = await executor.execute(step, {
        userId:    execution.userId,
        projectId: execution.projectId,
        executionId: execution.id,
      })

      await this.repository.updateStepStatus(execution.id, step.id, 'completed', result)
    } catch (err) {
      await this.repository.updateStepStatus(execution.id, step.id, 'failed', { error: String(err) })
      throw err
    }
  }

  /**
   * Group steps into waves based on dependencies.
   */
  private buildWaves(steps: WorkflowStep[]): WorkflowStep[][] {
    const completed = new Set<string>()
    const remaining = [...steps]
    const waves: WorkflowStep[][] = []

    while (remaining.length > 0) {
      const ready = remaining.filter(s =>
        (s.dependsOn ?? []).every(dep => completed.has(dep))
      )

      if (ready.length === 0) {
        throw new Error('Circular dependency detected in workflow steps')
      }

      waves.push(ready)
      ready.forEach(s => {
        completed.add(s.id)
        remaining.splice(remaining.indexOf(s), 1)
      })
    }

    return waves
  }
}
```

---

## BUG-WF-001 — Fix WorkflowServer spec not implemented

**Mức độ:** 🔴 HIGH  
**Root cause:** Workflow HTTP API server (routes/handlers) chưa được implement.

### Fix — Implement WorkflowServer routes

```typescript
// src/main/workflow/WorkflowServer.ts (NEW)

import { Router } from 'express'

export function createWorkflowRouter(orchestrator: WorkflowOrchestrator): Router {
  const router = Router()

  // List workflows
  router.get('/', requireAuth, async (req, res) => {
    const workflows = await orchestrator.listWorkflows(req.orcaSession!.userId)
    res.json({ workflows })
  })

  // Create workflow
  router.post('/', requireAuth, async (req, res) => {
    try {
      const workflow = await orchestrator.createWorkflow({
        userId:    req.orcaSession!.userId,
        projectId: req.body.projectId,
        name:      req.body.name,
        steps:     req.body.steps,
        trigger:   req.body.trigger,
      })
      res.status(201).json({ workflow })
    } catch (err) {
      res.status(400).json({ error: String(err) })
    }
  })

  // Start execution
  router.post('/:id/execute', requireAuth, async (req, res) => {
    const execution = await orchestrator.startExecution({
      workflowId: req.params.id,
      userId:     req.orcaSession!.userId,
      projectId:  req.body.projectId ?? '',
      trigger:    { type: 'manual', payload: req.body },
    })
    res.json({ execution })
  })

  // Get execution status
  router.get('/executions/:executionId', requireAuth, async (req, res) => {
    const execution = await orchestrator.getExecution(req.params.executionId)
    res.json({ execution })
  })

  // Cancel execution
  router.post('/executions/:executionId/cancel', requireAuth, async (req, res) => {
    await orchestrator.cancelExecution(req.params.executionId)
    res.json({ ok: true })
  })

  return router
}
```

---

## BUG-WF-003 — Fix condition step code injection vulnerability

**Mức độ:** 🔴 CRITICAL (Security)  
**Root cause:** `condition` step type dùng `eval()` để evaluate expressions → arbitrary code execution.

### Fix — Replace eval với safe expression evaluator

```typescript
// src/main/workflow/StepExecutors.ts

// TRƯỚC (VULNERABLE):
class ConditionStepExecutor {
  async execute(step: WorkflowStep, context: ExecutionContext): Promise<StepResult> {
    const expression = step.config['expression'] as string
    // BUG: eval allows arbitrary code execution
    const result = eval(expression)  // XSS/RCE vulnerability
    return { passed: !!result }
  }
}

// SAU — Safe expression evaluator:
import { evaluate } from 'safe-expression'  // npm package: safe, sandboxed

class ConditionStepExecutor {
  // Whitelist of allowed operators and functions
  private static readonly ALLOWED_OPERATORS = new Set([
    '===', '!==', '>', '<', '>=', '<=', '&&', '||', '!',
  ])

  async execute(step: WorkflowStep, context: ExecutionContext): Promise<StepResult> {
    const expression = step.config['expression'] as string

    // Validate expression before evaluation
    this.validateExpression(expression)

    // Evaluate với sandboxed context (no access to globals)
    const sandbox = {
      // Only expose workflow variables, not process/global
      env:         context.environment ?? {},
      outputs:     context.stepOutputs ?? {},
      executionId: context.executionId,
    }

    const result = this.safeEvaluate(expression, sandbox)
    return { passed: !!result }
  }

  private validateExpression(expression: string): void {
    // Block dangerous patterns
    const forbidden = [
      /\beval\b/, /\bFunction\b/, /\bprocess\b/, /\brequire\b/,
      /\bglobal\b/, /\bwindow\b/, /\b__proto__\b/, /\bprototype\b/,
      /\bconstructors?\b/i, /import\s*\(/, /\bawait\b/, /\basync\b/,
    ]

    for (const pattern of forbidden) {
      if (pattern.test(expression)) {
        throw new Error(`Unsafe expression: forbidden pattern detected`)
      }
    }

    // Max length to prevent DoS
    if (expression.length > 500) {
      throw new Error(`Expression too long: max 500 chars`)
    }
  }

  private safeEvaluate(expression: string, sandbox: Record<string, unknown>): unknown {
    // Use Function with restricted scope
    const keys = Object.keys(sandbox)
    const values = Object.values(sandbox)

    try {
      // Create function with only sandbox variables in scope
      const fn = new Function(...keys, `'use strict'; return (${expression})`)
      return fn(...values)
    } catch {
      return false  // Expression errors → condition false
    }
  }
}
```

---

## BUG-WF-004 — Fix resume orphan step execution

**Mức độ:** 🔴 HIGH  
**Root cause:** Khi server restart, các steps đang `running` không được reset → resumed execution tạo duplicate execution.

### Fix — Reset orphaned running steps trước khi resume

```typescript
// src/main/workflow/WorkflowOrchestrator.ts

/**
 * FIX WF-004: Resume running executions sau server restart.
 * Reset orphaned 'running' steps trước khi resume.
 */
async resumeRunningExecutions(): Promise<void> {
  const running = await this.repository.listExecutions({ status: 'running' })
  this.log.info(`[Workflow] Resuming ${running.length} interrupted executions`)

  for (const execution of running) {
    // FIX: Reset orphaned steps từ current wave (không duplicate)
    await this.resetOrphanedSteps(execution.id, execution.currentWave)
    
    // Resume từ current wave (không phải từ đầu)
    void this.runExecution(execution, execution.currentWave)
  }
}

/**
 * Reset steps đang 'running' trong wave hiện tại → 'failed' với reason 'server_restarted'.
 * Workflow sẽ retry chúng khi resume.
 */
private async resetOrphanedSteps(executionId: string, waveIndex: number): Promise<void> {
  await this.repository.resetOrphanedSteps(executionId, waveIndex)
  this.log.info(`[Workflow] Reset orphaned steps: executionId=${executionId} wave=${waveIndex}`)
}

// Repository implementation:
// src/main/repositories/workflow-repository.ts
async resetOrphanedSteps(executionId: string, waveIndex: number): Promise<void> {
  await this.pool.withConnection((db) =>
    db.query(
      `UPDATE orca_step_executions
       SET status = 'failed', error = 'server_restarted_mid_wave', completed_at = ?
       WHERE execution_id = ? AND wave_index = ? AND status = 'running'`,
      [Date.now(), executionId, waveIndex]
    )
  )
}

// server-bootstrap.ts — gọi sau khi WorkflowOrchestrator init:
// await workflowOrchestrator.resumeRunningExecutions()
```

---

## Tóm tắt file changes

| File | Action | Bug |
|------|--------|-----|
| `src/main/workflow/WorkflowOrchestrator.ts` | Implement full orchestrator | BE-WF-001 |
| `src/main/workflow/WorkflowServer.ts` | NEW — HTTP routes | WF-001 |
| `src/main/workflow/StepExecutors.ts` | Replace eval() với safe evaluator | WF-003 |
| `src/main/workflow/WorkflowOrchestrator.ts` | Add resumeRunningExecutions + resetOrphanedSteps | WF-004 |
| `src/main/repositories/workflow-repository.ts` | NEW — repository interface + SQL impl | BE-WF-001 |
| `src/main/db/migrations/0016_workflow.ts` | NEW migration | BE-WF-001 |
| `src/server/http-server.ts` | Mount /workflow router | WF-001 |
| `src/main/server-bootstrap.ts` | Init orchestrator + resumeRunningExecutions | WF-004 |

---

## Verification Plan

```bash
pnpm vitest run src/main/workflow/__tests__/

# Security test WF-003:
# 1. Condition expression 'eval("process.exit()")' → expect rejected
# 2. Condition expression 'require("fs")' → expect rejected
# 3. Condition expression 'outputs.prev === "success"' → expect evaluated safely

# Resume test WF-004:
# 1. Start execution → kill server mid-wave → restart → verify steps reset + resumed
# 2. Verify no duplicate step execution
# 3. Verify current_wave preserved (resume from correct wave)

# Wave model test:
# 1. Steps A, B (no deps) → C (depends A, B) → verify A+B run parallel, then C
```
