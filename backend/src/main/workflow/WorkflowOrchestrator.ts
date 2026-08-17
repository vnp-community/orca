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
import { Tracers } from '../../shared/trace/tracers'
import type { TraceSpan } from '../../shared/trace'
import type { IConnectionPool } from '../db/pool'
import type { BindValue } from '../db/types'
import type { ProjectServerRouter } from '../project/ProjectServerRouter'
import type { DAGBuilder } from './DAGBuilder'
// FIX §0: import CLASS thật thay vì tự định nghĩa alias Record<string, fn> trùng tên —
// WorkflowOrchestrator trước đây không bao giờ gọi trúng StepExecutors.execute() thật sự
// (xem SOLUTION-workflow-exact.md §0), khiến mọi step luôn throw UNSUPPORTED_STEP_TYPE.
import type { StepExecutors } from './StepExecutors'
import type {
  WorkflowDefinition,
  WorkflowExecution,
  WorkflowStep,
  StepOutput,
  ListExecutionsFilter,
} from './WorkflowTypes'

// (xoá hẳn export type StepExecutorFn / export type StepExecutors nội bộ — không còn dùng)

// ── DB row types ──────────────────────────────────────────────────────────────

type ExecutionRow = {
  id: string
  definitionSnapshot: string  // JSON — column: definition_snapshot
  status: string
  inputsJson: string          // JSON — column: inputs_json
  currentWave: number
  triggeredBy: string
  projectId: string | null
  startedAt: number | null
  completedAt: number | null
  pausedAt: number | null // [NEW BUG-BE-HLD-009]
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
    pausedAt: r.pausedAt ? new Date(r.pausedAt) : undefined, // [NEW BUG-BE-HLD-009]
    errorMessage: r.errorMessage ?? undefined,
    createdAt: new Date(r.createdAt),
  }
}

// ── Orchestrator ─────────────────────────────────────────────────────────────

export class WorkflowOrchestrator {
  /** Active abort controllers keyed by executionId */
  private readonly abortControllers = new Map<string, AbortController>()
  /** Span cha `workflow:execute` keyed by executionId — closed (ok/fail) on the
   *  3 terminal transitions (completed/failed/cancelled), then deleted. */
  private readonly rootSpans = new Map<string, TraceSpan>()
  // [NEW BUG-BE-HLD-009] executionIds có pause request đang chờ — kiểm tra ở đầu mỗi
  // vòng lặp wave trong runExecution(), KHÔNG abort AbortController (khác cancel()).
  private readonly pauseRequests = new Set<string>()

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

    // Span CHA — sống suốt vòng đời execution, id persist làm root_trace_id để
    // resumeRunningExecutions() tái tạo đúng span cha qua restart (CR-TRACE-000 §3.1 resume).
    const span = Tracers.workflowExecuteFlow.start({ executionId: id, projectId, triggeredBy })
    this.rootSpans.set(id, span)

    await this.persistExecution({
      id,
      definition,
      inputs,
      triggeredBy,
      projectId,
      now,
      rootTraceId: span.id,
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
    // rootTraceId truyền xuống runExecution() để mọi step span mang đúng parentTraceId
    void this.runExecution(execution, 0, span.id)

    return execution
  }

  /**
   * Get a workflow execution by ID.
   */
  async getExecution(executionId: string): Promise<WorkflowExecution | null> {
    const rows = await this.pool.withConnection((db) =>
      db.query<ExecutionRow>(
        `SELECT id,
                definition_snapshot as "definitionSnapshot",
                status,
                inputs_json         as "inputsJson",
                current_wave        as "currentWave",
                triggered_by        as "triggeredBy",
                project_id          as "projectId",
                started_at          as "startedAt",
                completed_at        as "completedAt",
                paused_at           as "pausedAt",
                error_message       as "errorMessage",
                created_at          as "createdAt"
         FROM orca_workflow_executions WHERE id = ?`,
        [executionId]
      )
    )
    if (!rows[0]) {return null}
    return rowToExecution(rows[0])
  }

  /**
   * List workflow executions with optional filters.
   */
  async listExecutions(filters: ListExecutionsFilter = {}): Promise<WorkflowExecution[]> {
    const clauses: string[] = []
    const params: BindValue[] = []

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
             definition_snapshot as "definitionSnapshot",
             status,
             inputs_json         as "inputsJson",
             current_wave        as "currentWave",
             triggered_by        as "triggeredBy",
             project_id          as "projectId",
             started_at          as "startedAt",
             completed_at        as "completedAt",
             paused_at           as "pausedAt",
             error_message       as "errorMessage",
             created_at          as "createdAt"
      FROM orca_workflow_executions ${where}
      ORDER BY created_at DESC LIMIT ?`

    const rows = await this.pool.withConnection((db) =>
      db.query<ExecutionRow>(sql, [...params, limit])
    )
    return rows.map(rowToExecution)
  }

  /**
   * User-triggered pause. Does NOT abort in-flight steps — the current wave (if
   * any is running) is allowed to finish; only the NEXT wave's dispatch is withheld.
   * Idempotent-safe against double-pause via the status guard below.
   *
   * @throws Error('EXECUTION_NOT_FOUND')          executionId doesn't exist
   * @throws Error('WORKFLOW_PAUSE_INVALID_STATE')  execution isn't currently 'running'
   */
  async pause(executionId: string): Promise<void> {
    const execution = await this.getExecution(executionId)
    if (!execution) {throw new Error(`EXECUTION_NOT_FOUND: ${executionId}`)}
    if (execution.status !== 'running') {
      throw new Error(
        `WORKFLOW_PAUSE_INVALID_STATE: cannot pause execution "${executionId}" in status "${execution.status}" (expected "running")`
      )
    }
    this.pauseRequests.add(executionId)
    console.log(`[WorkflowOrchestrator] Pause requested for execution ${executionId} — will stop before next wave`)
  }

  /**
   * User-triggered resume of a PAUSED execution — distinct from resumeRunningExecutions()
   * (internal crash-recovery, bootstrap-only, scans ALL status='running' executions).
   * This method targets exactly one execution and only accepts status='paused'.
   *
   * @throws Error('EXECUTION_NOT_FOUND')           executionId doesn't exist
   * @throws Error('WORKFLOW_RESUME_INVALID_STATE')  execution isn't currently 'paused'
   */
  async resumeFromPause(executionId: string): Promise<void> {
    const execution = await this.getExecution(executionId)
    if (!execution) {throw new Error(`EXECUTION_NOT_FOUND: ${executionId}`)}
    if (execution.status !== 'paused') {
      throw new Error(
        `WORKFLOW_RESUME_INVALID_STATE: cannot resume execution "${executionId}" from status "${execution.status}" (expected "paused")`
      )
    }

    console.log(`[WorkflowOrchestrator] User-resuming paused execution ${executionId} from wave ${execution.currentWave}`)

    // Re-read root_trace_id like resumeRunningExecutions() does — rootSpans is in-memory
    // only, so a server restart WHILE paused (paused executions are intentionally excluded
    // from resumeRunningExecutions()'s bootstrap scan) would otherwise lose the parent span id.
    let span = this.rootSpans.get(executionId)
    if (!span) {
      const rows = await this.pool.withConnection((db) =>
        db.query(`SELECT root_trace_id as "rootTraceId" FROM orca_workflow_executions WHERE id = ?`, [executionId])
      )
      const rootTraceId = (rows[0] as { rootTraceId: string | null } | undefined)?.rootTraceId ?? undefined
      span = Tracers.workflowExecuteFlow.start(
        { executionId, projectId: execution.projectId, resumedFromPause: true },
        rootTraceId ? { id: rootTraceId } : undefined
      )
      this.rootSpans.set(executionId, span)
    }

    await this.clearPausedAt(executionId)
    // runExecution() calls markExecutionRunning() internally — transitions status paused → running.
    void this.runExecution(execution, execution.currentWave, span.id)
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
    this.pauseRequests.delete(executionId) // [NEW BUG-BE-HLD-009] cancel wins over a pending pause request
    // Update status in DB
    await this.pool.withConnection((db) =>
      db.query(
        `UPDATE orca_workflow_executions SET status = 'cancelled', completed_at = ? WHERE id = ?`,
        [Date.now(), executionId]
      )
    )
    // 3rd terminal transition (with completed/failed) that must close+delete rootSpans —
    // otherwise a cancelled execution's parent span never closes and the map leaks.
    const span = this.rootSpans.get(executionId)
    if (span) {
      span.fail('EXECUTION_CANCELLED', { status: 'cancelled' })
      this.rootSpans.delete(executionId)
    }
  }

  /**
   * Resume any executions that were marked 'running' when the server restarted.
   * Called at bootstrap to recover orphaned executions.
   */
  async resumeRunningExecutions(): Promise<void> {
    const running = await this.listExecutions({ status: 'running' })
    for (const execution of running) {
      console.log(`[WorkflowOrchestrator] Resuming execution ${execution.id} from wave ${execution.currentWave}`)
      // Đọc lại root_trace_id đã persist — resume() giữ nguyên id cha qua restart
      // (CR-TRACE-000 §3.1), để step span cũ (trước restart) và mới nhóm chung 1 execution.
      const rows = await this.pool.withConnection((db) =>
        db.query(`SELECT root_trace_id as "rootTraceId" FROM orca_workflow_executions WHERE id = ?`, [
          execution.id,
        ])
      )
      const rootTraceId = (rows[0] as { rootTraceId: string | null } | undefined)?.rootTraceId ?? undefined
      const span = Tracers.workflowExecuteFlow.start(
        { executionId: execution.id, projectId: execution.projectId, resumed: true },
        rootTraceId ? { id: rootTraceId } : undefined
      )
      this.rootSpans.set(execution.id, span)
      void this.runExecution(execution, execution.currentWave, span.id)
    }
  }

  // ── Private: execution engine ─────────────────────────────────────────────

  private async runExecution(
    execution: WorkflowExecution,
    startWave = 0,
    rootTraceId?: string // [NEW] id của span cha workflow:execute — dùng làm parentTraceId cho mọi step
  ): Promise<void> {
    const controller = new AbortController()
    this.abortControllers.set(execution.id, controller)

    try {
      await this.markExecutionRunning(execution.id)

      const waves = this.dagBuilder.buildWaves(execution.definition.steps)

      for (let waveIndex = startWave; waveIndex < waves.length; waveIndex++) {
        if (controller.signal.aborted) {
          return
        }

        // [NEW BUG-BE-HLD-009] Check pause request BEFORE dispatching the next wave. The
        // previous wave's steps (if any) have already been awaited by this point in the loop —
        // they finish normally; only this wave's dispatch is withheld. current_wave stays at
        // the last value written by updateCurrentWave() (the last COMPLETED wave), matching
        // "giữ nguyên state DB hiện tại, không phải rollback".
        if (this.pauseRequests.has(execution.id)) {
          this.pauseRequests.delete(execution.id)
          await this.markExecutionPaused(execution.id)
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
            return this.executeStep(step, execution, controller.signal, rootTraceId)
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
    signal: AbortSignal,
    rootTraceId?: string // [NEW] id của span cha workflow:execute — dùng làm parentTraceId
  ): Promise<StepOutput> {
    const interpolatedStep = this.interpolateStep(step, execution.inputs)

    // FIX §0: gọi thẳng entry point duy nhất của class StepExecutors — nó tự dispatch theo
    // step.config.type nội bộ (executeByType) và tự throw UNSUPPORTED_STEP_TYPE nếu cần,
    // nên không còn cần bước tra map + kiểm tra !executor ở đây.
    // Span CON độc lập — id riêng (không resume), mang field parentTraceId để
    // TracePanel group N step song song dưới cùng 1 execution (BL-WF-02 design).
    const stepSpan = Tracers.workflowStepFlow.start({
      parentTraceId: rootTraceId,
      executionId: execution.id,
      stepId: step.id,
      stepType: interpolatedStep.config.type as string,
    })

    try {
      await this.persistStepStart(execution.id, step.id)

      const output = await this.stepExecutors.execute(
        interpolatedStep,
        execution.inputs,
        signal,
        stepSpan.id,
        execution.triggeredBy // [NEW BUG-BE-HLD-008] forward — dùng để ProviderResolver áp user-scope priority
      )

      await this.persistStepComplete(execution.id, step.id, output)

      stepSpan.ok({ exitCode: output.exitCode })
      return output
    } catch (err) {
      stepSpan.fail(err, { stepId: step.id })
      throw err
    }
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
    rootTraceId?: string
  }): Promise<void> {
    await this.pool.withConnection((db) =>
      db.query(
        `INSERT INTO orca_workflow_executions
           (id, definition_snapshot, status, inputs_json, current_wave, triggered_by, project_id, created_at, root_trace_id)
         VALUES (?, ?, 'pending', ?, 0, ?, ?, ?, ?)`,
        [
          params.id,
          JSON.stringify(params.definition),
          JSON.stringify(params.inputs),
          params.triggeredBy,
          params.projectId ?? null,
          params.now,
          params.rootTraceId ?? null,
        ]
      )
    )
  }

  // [NEW BUG-BE-HLD-009]
  private async markExecutionPaused(executionId: string): Promise<void> {
    await this.pool.withConnection((db) =>
      db.query(
        `UPDATE orca_workflow_executions SET status = 'paused', paused_at = ? WHERE id = ?`,
        [Date.now(), executionId]
      )
    )
    // 'paused' is NOT a terminal status (unlike completed/failed/cancelled) — the root span
    // stays open in rootSpans; TracePanel keeps grouping steps under it after resume.
  }

  private async clearPausedAt(executionId: string): Promise<void> {
    await this.pool.withConnection((db) =>
      db.query(`UPDATE orca_workflow_executions SET paused_at = NULL WHERE id = ?`, [executionId])
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
    const span = this.rootSpans.get(executionId)
    if (span) {
      span.ok({ status: 'completed' })
      this.rootSpans.delete(executionId)
    }
  }

  private async markExecutionFailed(executionId: string, errorMessage: string): Promise<void> {
    await this.pool.withConnection((db) =>
      db.query(
        `UPDATE orca_workflow_executions
         SET status = 'failed', completed_at = ?, error_message = ? WHERE id = ?`,
        [Date.now(), errorMessage, executionId]
      )
    )
    const span = this.rootSpans.get(executionId)
    if (span) {
      span.fail(errorMessage, { status: 'failed' })
      this.rootSpans.delete(executionId)
    }
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
