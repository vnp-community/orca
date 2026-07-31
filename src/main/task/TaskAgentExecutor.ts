/**
 * TaskAgentExecutor — Executes an AI agent on a task with lifecycle management (TDD-18)
 *
 * Execution flow:
 * 1. Check permission: resolvePermission(userId, taskId) — need 'execute' or 'manage'
 * 2. Get task → build prompt (from promptTemplate or auto-generated)
 * 3. Update status → 'in_progress'
 * 4. Spawn agent via agentSpawner.spawn({ projectId, userId, command: prompt })
 * 5. On success: update status → 'review', add 'activity' comment
 * 6. On error: update status → 'blocked', add error comment
 *
 * @module main/task/TaskAgentExecutor
 */

import type { TaskService } from './TaskService'
import type { ProfileAwareAgentSpawner } from '../project/ProfileAwareAgentSpawner'
import type { TaskGrantService } from './TaskGrantService'
import type { OrcaTask } from '../../shared/task-types'
import { TASK_PERMISSION_ORDER } from '../../shared/task-types'

/** Minimum permission required to execute an agent on a task */
const MIN_EXECUTE_LEVEL = TASK_PERMISSION_ORDER['execute'] // 4

export interface ExecuteTaskParams {
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
    private readonly grantService: TaskGrantService
  ) {}

  /**
   * Execute agent on a task.
   *
   * @throws Error('TASK_PERMISSION_DENIED') if user lacks 'execute' or 'manage'
   * @throws Error('TASK_NOT_FOUND') if task does not exist
   */
  async executeTask(params: ExecuteTaskParams): Promise<void> {
    const { taskId, projectId, userId, worktreePath } = params

    // 1. Check permission
    const perm = await this.grantService.resolvePermission(userId, taskId)
    const permLevel = perm ? (TASK_PERMISSION_ORDER[perm] ?? 0) : 0
    if (permLevel < MIN_EXECUTE_LEVEL) {
      throw new Error(
        `TASK_PERMISSION_DENIED: user "${userId}" needs "execute" or "manage" to run agent on task "${taskId}"`
      )
    }

    // 2. Get task
    const task = await this.taskService.get(taskId)
    if (!task) throw new Error(`TASK_NOT_FOUND: ${taskId}`)

    // 3. Build prompt
    const prompt = this.buildPrompt(task)

    // 4. Update status → in_progress
    await this.taskService.update(taskId, { status: 'in_progress' })
    await this.taskService.addComment(
      taskId,
      userId,
      `Agent execution started by ${userId}`,
      'activity'
    )

    // 5. Spawn agent
    try {
      await this.agentSpawner.spawn({
        projectId,
        userId,
        command: prompt,
        workdir: worktreePath,
        extraEnv: params.accountId ? { ORCA_ACCOUNT_ID: params.accountId } : undefined,
      })

      // 6. Success: update status → review
      await this.taskService.update(taskId, { status: 'review' })
      await this.taskService.addComment(
        taskId,
        userId,
        `Agent execution completed successfully`,
        'activity'
      )
    } catch (err) {
      // 6. Error: update status → blocked
      const errMsg = err instanceof Error ? err.message : String(err)
      await this.taskService.update(taskId, { status: 'blocked' }).catch(() => {})
      await this.taskService.addComment(
        taskId,
        userId,
        `Agent execution failed: ${errMsg}`,
        'activity'
      ).catch(() => {})
      throw err
    }
  }

  /**
   * Build the agent prompt from a task's context.
   * Uses task.promptTemplate with ${task.*} interpolation if available,
   * otherwise auto-generates from title + description + aiContext.
   */
  buildPrompt(task: OrcaTask): string {
    if (task.promptTemplate) {
      return task.promptTemplate.replace(/\$\{task\.([^}]+)\}/g, (_, key: string) => {
        const val = (task as unknown as Record<string, unknown>)[key]
        return val !== undefined ? String(val) : `\${task.${key}}`
      })
    }

    // Auto-generated prompt
    const lines: string[] = [`# Task: ${task.title}`]
    if (task.description) lines.push(`\n## Description\n${task.description}`)
    if (task.aiContext) lines.push(`\n## AI Context\n${task.aiContext}`)
    lines.push(`\n## Instructions`)
    lines.push(
      `Complete the task described above. When finished, the task status will be moved to "review".`
    )
    return lines.join('\n')
  }
}
