/**
 * WorkflowOrchestrator — DAG-based multi-server workflow execution (TDD-17)
 *
 * Execution flow:
 * execute() → persist(status=pending) → markRunning → buildWaves →
 * forEach wave: Promise.allSettled(executeSteps) → updateWave →
 * markCompleted/Failed
 *
 * Key design decisions:
 * - Each execution has an AbortController so cancel() can abort in-flight waves
 * - Steps in the same wave run via Promise.allSettled (all attempted even if one fails)
 * - Input interpolation: ${inputs.varName} replaced in step config strings
 * - DB persistence follows sql-repository pattern: pool.withConnection()
 *
 * @module main/workflow/WorkflowOrchestrator
 */

import { randomUUID } from 'node:crypto'
import type { IConnectionPool } from '../db/pool'
import type { ProjectServerRouter } from '../project/ProjectServerRouter'
import type { DAGBuilder } from './DAGBuilder'
import type {
  WorkflowDefinition,
  WorkflowExecution,
  WorkflowStep,
  StepOutput,
  ListExecutionsFilter,
} from './WorkflowTypes'

// ── Step executors type ───────────────────────────────────────────────────────

export type StepExecutorFn = (
  step: WorkflowStep,
  inputs: Record<string, unknown>,
  signal: AbortSignal
) => Promise<StepOutput>

export type StepExecutors = Record<string, StepExecutorFn>

// ── DB row types ──────────────────────────────────────────────────────────────

interface ExecutionRow {
  id: string
  definitionSnapshot: string  // JSON — column: definition_snapshot
  status: string
  inputsJson: string          // JSON — column: inputs_json
  currentWave: number
  triggeredBy: string
  projectId: string | null
  startedAt: number | null
  completedAt: number | null
  errorMessage: string | null
  createdAt: number
}

function rowToExecution(r: ExecutionRow): WorkflowExecution {
  return {
    id: r.id,
    definition: JSON.parse(r.definitionSnapshot) as WorkflowDefinition,
    status: r.status as WorkflowExecution['status'],
    inputs: JSON.parse(r.inputsJson) as Record<string, unknown>,
    currentWave: r.currentWave,
    triggeredBy: r.triggeredBy,
    projectId: r.projectId ?? undefined,
    startedAt: r.startedAt ? new Date(r.startedAt) : undefined,
    completedAt: r.completedAt ? new Date(r.completedAt) : undefined,
    errorMessage: r.errorMessage ?? undefined,
    createdAt: new Date(r.createdAt),
  }
}

// ── Orchestrator ─────────────────────────────────────────────────────────────

export class WorkflowOrchestrator {
  /** Active abort controllers keyed by executionId */
  private readonly abortControllers = new Map<string, AbortController>()

  constructor(
    private readonly pool: IConnectionPool,
    private readonly dagBuilder: DAGBuilder,
    private readonly stepExecutors: StepExecutors,
    private readonly router: ProjectServerRouter
  ) {}

  // ── Public API ────────────────────────────────────────────────────────────

  /**
   * Execute a workflow definition asynchronously.
   * Persists the execution immediately, then runs steps in the background.
   *
   * @returns The persisted WorkflowExecution (status='pending' or 'running')
   */
  async execute(
    definition: WorkflowDefinition,
    inputs: Record<string, unknown>,
    triggeredBy: string,
    projectId?: string
  ): Promise<WorkflowExecution> {
    const id = randomUUID()
    const now = Date.now()

    await this.persistExecution({
      id,
      definition,
      inputs,
      triggeredBy,
      projectId,
      now,
    })

    const execution: WorkflowExecution = {
      id,
      definition,
      status: 'pending',
      inputs,
      currentWave: 0,
      triggeredBy,
      projectId,
      createdAt: new Date(now),
    }

    // Run asynchronously — caller gets the pending execution immediately
    void this.runExecution(execution)

    return execution
  }

  /**
   * Get a workflow execution by ID.
   */
  async getExecution(executionId: string): Promise<WorkflowExecution | null> {
    const rows = await this.pool.withConnection((db) =>
      db.query<ExecutionRow>(
        `SELECT id,
                definition_snapshot as definitionSnapshot,
                status,
                inputs_json         as inputsJson,
                current_wave        as currentWave,
                triggered_by        as triggeredBy,
                project_id          as projectId,
                started_at          as startedAt,
                completed_at        as completedAt,
                error_message       as errorMessage,
                created_at          as createdAt
         FROM orca_workflow_executions WHERE id = ?`,
        [executionId]
      )
    )
    if (!rows[0]) return null
    return rowToExecution(rows[0])
  }

  /**
   * List workflow executions with optional filters.
   */
  async listExecutions(filters: ListExecutionsFilter = {}): Promise<WorkflowExecution[]> {
    const clauses: string[] = []
    const params: unknown[] = []

    if (filters.projectId) {
      clauses.push('project_id = ?')
      params.push(filters.projectId)
    }
    if (filters.triggeredBy) {
      clauses.push('triggered_by = ?')
      params.push(filters.triggeredBy)
    }
    if (filters.status) {
      clauses.push('status = ?')
      params.push(filters.status)
    }

    const where = clauses.length > 0 ? `WHERE ${clauses.join(' AND ')}` : ''
    const limit = filters.limit ?? 100
    const sql = `
      SELECT id,
             definition_snapshot as definitionSnapshot,
             status,
             inputs_json         as inputsJson,
             current_wave        as currentWave,
             triggered_by        as triggeredBy,
             project_id          as projectId,
             started_at          as startedAt,
             completed_at        as completedAt,
             error_message       as errorMessage,
             created_at          as createdAt
      FROM orca_workflow_executions ${where}
      ORDER BY created_at DESC LIMIT ?`

    const rows = await this.pool.withConnection((db) =>
      db.query<ExecutionRow>(sql, [...params, limit])
    )
    return rows.map(rowToExecution)
  }

  /**
   * Cancel a running execution by aborting its AbortController.
   */
  async cancel(executionId: string): Promise<void> {
    const controller = this.abortControllers.get(executionId)
    if (controller) {
      controller.abort()
      this.abortControllers.delete(executionId)
    }
    // Update status in DB
    await this.pool.withConnection((db) =>
      db.query(
        `UPDATE orca_workflow_executions SET status = 'cancelled', completed_at = ? WHERE id = ?`,
        [Date.now(), executionId]
      )
    )
  }

  /**
   * Resume any executions that were marked 'running' when the server restarted.
   * Called at bootstrap to recover orphaned executions.
   */
  async resumeRunningExecutions(): Promise<void> {
    const running = await this.listExecutions({ status: 'running' })
    for (const execution of running) {
      console.log(`[WorkflowOrchestrator] Resuming execution ${execution.id} from wave ${execution.currentWave}`)
      void this.runExecution(execution, execution.currentWave)
    }
  }

  // ── Private: execution engine ─────────────────────────────────────────────

  private async runExecution(execution: WorkflowExecution, startWave = 0): Promise<void> {
    const controller = new AbortController()
    this.abortControllers.set(execution.id, controller)

    try {
      await this.markExecutionRunning(execution.id)

      const waves = this.dagBuilder.buildWaves(execution.definition.steps)

      for (let waveIndex = startWave; waveIndex < waves.length; waveIndex++) {
        if (controller.signal.aborted) {
          return
        }

        await this.updateCurrentWave(execution.id, waveIndex)
        const wave = waves[waveIndex]

        const results = await Promise.allSettled(
          wave.map(async (step) => {
            // FIX TASK-WF-002: On resume (startWave > 0 or execution was 'running' at bootstrap),
            // check if this individual step already completed in a previous run.
            // This prevents re-running completed steps when a server crash interrupted mid-wave.
            if (startWave > 0 || execution.status === 'running') {
              const rows = await this.pool.withConnection((db) =>
                db.query(
                  `SELECT status FROM orca_workflow_step_executions
                   WHERE execution_id = ? AND step_id = ?`,
                  [execution.id, step.id]
                )
              ).catch(() => null)

              const stepRecord = rows?.[0] as { status: string } | undefined
              if (stepRecord?.status === 'completed') {
                console.log(`[WorkflowOrchestrator] Skipping already-completed step ${step.id} (resume)`)
                return { exitCode: 0, data: { skippedOnResume: true } }
              }
            }
            return this.executeStep(step, execution, controller.signal)
          })
        )

        // Check if any step failed (and continueOnError is not set)
        let shouldFail = false
        let firstError: string | undefined

        for (let i = 0; i < results.length; i++) {
          const result = results[i]
          if (result.status === 'rejected') {
            const step = wave[i]
            if (!step.continueOnError) {
              shouldFail = true
              firstError = result.reason instanceof Error ? result.reason.message : String(result.reason)
              break
            }
          } else if (result.value.exitCode !== 0) {
            const step = wave[i]
            if (!step.continueOnError) {
              shouldFail = true
              firstError = `Step ${step.id} exited with code ${result.value.exitCode}`
              break
            }
          }
        }

        if (shouldFail) {
          await this.markExecutionFailed(execution.id, firstError ?? 'Unknown error')
          return
        }
      }

      await this.markExecutionCompleted(execution.id)
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err)
      await this.markExecutionFailed(execution.id, message).catch(() => {})
    } finally {
      this.abortControllers.delete(execution.id)
    }
  }

  private async executeStep(
    step: WorkflowStep,
    execution: WorkflowExecution,
    signal: AbortSignal
  ): Promise<StepOutput> {
    const interpolatedStep = this.interpolateStep(step, execution.inputs)
    const executor = this.stepExecutors[interpolatedStep.config.type as string]

    if (!executor) {
      throw new Error(`UNSUPPORTED_STEP_TYPE: ${interpolatedStep.config.type}`)
    }

    await this.persistStepStart(execution.id, step.id)

    const output = await executor(interpolatedStep, execution.inputs, signal)

    await this.persistStepComplete(execution.id, step.id, output)

    return output
  }

  // ── Input interpolation ───────────────────────────────────────────────────

  /**
   * Replace ${inputs.varName} placeholders in step config string values.
   */
  private interpolateStep(step: WorkflowStep, inputs: Record<string, unknown>): WorkflowStep {
    const interpolated = JSON.parse(JSON.stringify(step)) as WorkflowStep
    interpolated.config = this.interpolateObject(interpolated.config, inputs) as typeof interpolated.config
    return interpolated
  }

  private interpolateValue(value: unknown, inputs: Record<string, unknown>): unknown {
    if (typeof value === 'string') {
      return value.replace(/\$\{inputs\.([^}]+)\}/g, (_, key: string) => {
        const v = inputs[key]
        return v !== undefined ? String(v) : `\${inputs.${key}}`
      })
    }
    if (Array.isArray(value)) {
      return value.map((v) => this.interpolateValue(v, inputs))
    }
    if (value !== null && typeof value === 'object') {
      return this.interpolateObject(value as Record<string, unknown>, inputs)
    }
    return value
  }

  private interpolateObject(
    obj: Record<string, unknown>,
    inputs: Record<string, unknown>
  ): Record<string, unknown> {
    const result: Record<string, unknown> = {}
    for (const [key, value] of Object.entries(obj)) {
      result[key] = this.interpolateValue(value, inputs)
    }
    return result
  }

  // ── DB persistence ────────────────────────────────────────────────────────

  private async persistExecution(params: {
    id: string
    definition: WorkflowDefinition
    inputs: Record<string, unknown>
    triggeredBy: string
    projectId?: string
    now: number
  }): Promise<void> {
    await this.pool.withConnection((db) =>
      db.query(
        `INSERT INTO orca_workflow_executions
           (id, definition_snapshot, status, inputs_json, current_wave, triggered_by, project_id, created_at)
         VALUES (?, ?, 'pending', ?, 0, ?, ?, ?)`,
        [
          params.id,
          JSON.stringify(params.definition),
          JSON.stringify(params.inputs),
          params.triggeredBy,
          params.projectId ?? null,
          params.now,
        ]
      )
    )
  }

  private async markExecutionRunning(executionId: string): Promise<void> {
    await this.pool.withConnection((db) =>
      db.query(
        `UPDATE orca_workflow_executions SET status = 'running', started_at = ? WHERE id = ?`,
        [Date.now(), executionId]
      )
    )
  }

  private async markExecutionCompleted(executionId: string): Promise<void> {
    await this.pool.withConnection((db) =>
      db.query(
        `UPDATE orca_workflow_executions SET status = 'completed', completed_at = ? WHERE id = ?`,
        [Date.now(), executionId]
      )
    )
  }

  private async markExecutionFailed(executionId: string, errorMessage: string): Promise<void> {
    await this.pool.withConnection((db) =>
      db.query(
        `UPDATE orca_workflow_executions
         SET status = 'failed', completed_at = ?, error_message = ? WHERE id = ?`,
        [Date.now(), errorMessage, executionId]
      )
    )
  }

  private async updateCurrentWave(executionId: string, wave: number): Promise<void> {
    await this.pool.withConnection((db) =>
      db.query(
        `UPDATE orca_workflow_executions SET current_wave = ? WHERE id = ?`,
        [wave, executionId]
      )
    )
  }

  private async persistStepStart(executionId: string, stepId: string): Promise<void> {
    const stepInstId = `${executionId}:${stepId}`
    await this.pool.withConnection((db) =>
      db.query(
        `INSERT OR REPLACE INTO orca_workflow_step_executions
           (id, execution_id, step_id, status, started_at)
         VALUES (?, ?, ?, 'running', ?)`,
        [stepInstId, executionId, stepId, Date.now()]
      )
    )
  }

  private async persistStepComplete(
    executionId: string,
    stepId: string,
    output: StepOutput
  ): Promise<void> {
    const stepInstId = `${executionId}:${stepId}`
    await this.pool.withConnection((db) =>
      db.query(
        `UPDATE orca_workflow_step_executions
         SET status = ?, completed_at = ?, output_json = ?, error_message = ?
         WHERE id = ?`,
        [
          output.exitCode === 0 ? 'completed' : 'failed',
          Date.now(),
          JSON.stringify({ stdout: output.stdout, data: output.data }),
          output.stderr ?? null,
          stepInstId,
        ]
      )
    )
  }
}
