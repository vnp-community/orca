/**
 * TaskAgentExecutor — Executes an AI agent on a task with lifecycle management (TDD-18)
 *
 * Execution flow:
 * 1. Check permission: resolvePermission(userId, taskId) — need 'execute' or 'manage'
 * 2. Get task, then route (docs/guides/task-automation-orchestration-integration.md §9.2):
 *    (a) no subtasks and no dependency edges → single-agent spawn (below)
 *    (b) has subtasks and/or dependency edges → dispatchToOrchestration() (TaskOrchestrationBridge)
 * 3a. Build prompt (from promptTemplate or auto-generated) → update status → 'in_progress'
 * 4a. Spawn agent via agentSpawner.spawn({ projectId, userId, command: prompt })
 * 5a. On success: update status → 'review', persist agentSessionId, add 'activity' comment
 * 6a. On error: update status → 'blocked', add error comment
 *
 * @module main/task/TaskAgentExecutor
 */

import type { TaskService } from './TaskService'
import type { ProfileAwareAgentSpawner, AgentSpawnOptions } from '../project/ProfileAwareAgentSpawner'
import type { TaskGrantService } from './TaskGrantService'
import type { OrcaTask } from '../../shared/task-types'
import { TASK_PERMISSION_ORDER } from '../../shared/task-types'
import { Tracers } from '../../shared/trace/tracers'
import { buildTaskAgentPrompt } from './task-agent-prompt'
import { recordAgentSessionCompletion } from './task-execution-result-listener'
import type { TaskOrchestrationBridge } from './TaskOrchestrationBridge'

/** Minimum permission required to execute an agent on a task */
const MIN_EXECUTE_LEVEL = TASK_PERMISSION_ORDER['execute'] // 4

export type ExecuteTaskParams = {
  taskId: string
  projectId: string
  userId: string
  worktreePath: string
  accountId?: string
}

export class TaskAgentExecutor {
  constructor(
    private readonly taskService: TaskService,
    private readonly agentSpawner: ProfileAwareAgentSpawner,
    private readonly grantService: TaskGrantService,
    // Why: optional so existing 3-arg call sites (server-bootstrap.ts) keep
    // compiling until the orchestration runtime is wired in — see
    // dispatchToOrchestration()'s guard below.
    private readonly orchestrationBridge?: TaskOrchestrationBridge
  ) {}

  /**
   * Execute agent on a task.
   *
   * @throws Error('TASK_PERMISSION_DENIED') if user lacks 'execute' or 'manage'
   * @throws Error('TASK_NOT_FOUND') if task does not exist
   */
  async executeTask(params: ExecuteTaskParams): Promise<void> {
    const { taskId, projectId, userId, worktreePath } = params
    // [CR-TRACE-018] taskGraph:execute is always a fresh root span — it does NOT resume from
    // any caller-supplied id (this is a distinct root operation, one per task.execute call).
    // It owns permission-check + build-prompt + the spawn() call, then forwards its OWN
    // span.id into agentSpawner.spawn() so agentOrch:spawn (TASK-BE-002.2, the sole canonical
    // span wrapping spawn()) resumes into THIS chain instead of opening a competing one.
    const span = Tracers.taskGraphExecuteFlow.start({ taskId, projectId, userId })

    try {
      // 1. Check permission — resolvePermission() already owns taskGraph:grantResolve
      // (TASK-BE-018.3); step() here only records the outcome on executeTask()'s own timeline.
      const perm = await this.grantService.resolvePermission(userId, taskId)
      const permLevel = perm ? (TASK_PERMISSION_ORDER[perm] ?? 0) : 0
      span.step('permission-check', { permLevel, permission: perm ?? 'none' })
      if (permLevel < MIN_EXECUTE_LEVEL) {
        span.fail('TASK_PERMISSION_DENIED', { userId, taskId })
        throw new Error(
          `TASK_PERMISSION_DENIED: user "${userId}" needs "execute" or "manage" to run agent on task "${taskId}"`
        )
      }

      // 2. Get task
      const task = await this.taskService.get(taskId)
      if (!task) {
        span.fail('TASK_NOT_FOUND', { taskId })
        throw new Error(`TASK_NOT_FOUND: ${taskId}`)
      }

      // 2b. Route (a) single-agent spawn vs (b) multi-agent orchestration —
      // §9.2: a task with subtasks or dependency edges needs coordinated
      // multi-step execution, which only the orchestration coordinator provides.
      const children = await this.taskService.getChildren(taskId)
      const deps = await this.taskService.getDependencies(taskId)
      const isComplex = children.length > 0 || deps.length > 0

      if (isComplex) {
        span.step('orchestration-dispatch', { childCount: children.length, depCount: deps.length })
        try {
          await this.dispatchToOrchestration(taskId, worktreePath)
          span.ok({ status: 'dispatched', mode: 'orchestration' })
        } catch (err) {
          span.fail(err, { mode: 'orchestration' })
          throw err
        }
        return
      }

      // 3. Build prompt — in-memory transform, no dedicated step (CR-TRACE-000 §5)
      const prompt = this.buildPrompt(task)

      // 4. Update status → in_progress
      await this.taskService.update(taskId, { status: 'in_progress' })
      await this.taskService.addComment(
        taskId,
        userId,
        `Agent execution started by ${userId}`,
        'activity'
      )

      // 5. Spawn agent — network hop, forward span.id as traceId so agentOrch:spawn
      // (TASK-BE-002.2 — sole canonical span wrapping spawn()) RESUMES with this same id
      // instead of opening an independent one. This branch does NOT go through
      // profile:agentSpawnRoute (that span only exists on the project.agentSpawn RPC
      // path, see TASK-BE-015.4) — taskGraph:execute resumes straight into agentOrch:spawn.
      try {
        span.step('agent-spawn', { worktreePath, hasAccountOverride: !!params.accountId })
        // AgentSpawnOptions.traceId is owned by TASK-BE-002.2 (SOL-BE-TRACE-018 Known
        // Conflicts resolution) — out of scope to add to ProfileAwareAgentSpawner.ts from
        // this task. Widening the local options type keeps this forward-compatible without
        // touching that file: once traceId + resume logic land there, this starts resuming
        // into agentOrch:spawn automatically; until then spawn() just ignores the extra field.
        const spawnOptions: AgentSpawnOptions & { traceId?: string } = {
          projectId,
          userId,
          command: prompt,
          workdir: worktreePath,
          extraEnv: params.accountId ? { ORCA_ACCOUNT_ID: params.accountId } : undefined,
          traceId: span.id,
        }
        const spawnResult = await this.agentSpawner.spawn(spawnOptions)

        // 6. Success: update status → review, persist the agent session id
        // this task ran under (migration 0016 — Source→Plan→Execute linkage)
        await recordAgentSessionCompletion(this.taskService, taskId, {
          status: 'review',
          agentSessionId: spawnResult.sessionId,
        })
        await this.taskService.addComment(
          taskId,
          userId,
          `Agent execution completed successfully`,
          'activity'
        )
        span.ok({ status: 'review' })
      } catch (err) {
        // 6. Error: update status → blocked
        const errMsg = err instanceof Error ? err.message : String(err)
        await recordAgentSessionCompletion(this.taskService, taskId, { status: 'blocked' }).catch(() => {})
        await this.taskService.addComment(
          taskId,
          userId,
          `Agent execution failed: ${errMsg}`,
          'activity'
        ).catch(() => {})
        span.fail(err, { status: 'blocked' })
        throw err
      }
    } catch (err) {
      // permission-check/task-not-found already called span.fail() above; this outer
      // catch only re-throws so we never double-fail the same span.
      throw err
    }
  }

  /**
   * Build the agent prompt from a task's context.
   * Uses task.promptTemplate with ${task.*} interpolation if available,
   * otherwise auto-generates from title + description + aiContext.
   */
  buildPrompt(task: OrcaTask): string {
    return buildTaskAgentPrompt(task)
  }

  /**
   * (b) Complex-task path: dispatch to the multi-agent orchestration
   * coordinator instead of a single spawn(). Seeding + write-back wiring
   * lives in TaskOrchestrationBridge (kept out of this file per AGENTS.md's
   * "Do Not Disable Max Lines" rule — see that file's header).
   */
  async dispatchToOrchestration(taskId: string, worktreePath: string): Promise<void> {
    if (!this.orchestrationBridge) {
      throw new Error(
        `TASK_ORCHESTRATION_UNAVAILABLE: no orchestration runtime wired for task "${taskId}" — ` +
          `TaskAgentExecutor needs a TaskOrchestrationBridge (see TaskOrchestrationBridge.ts) to run the complex-task path`
      )
    }
    await this.orchestrationBridge.dispatch(taskId, { worktree: worktreePath })
  }
}
