// src/main/task/commit-task-advance.ts
import type { TaskService } from './TaskService'
import type { TaskGrantService } from './TaskGrantService'

const TASK_REF_REGEX = /(?:closes?\s+)?#(TG-[\w-]+)/gi

const EDIT_LEVEL_OR_HIGHER = new Set(['edit', 'execute', 'manage'])

/**
 * Parse commit message for task refs and advance matching tasks to 'review'.
 * Called after git commit on Orca Server.
 *
 * @param commitMsg - Full commit message string
 * @param projectId - Project context (tasks in other projects are skipped)
 * @param userId - User who made the commit
 * @param taskService - TaskService with findByRef, update, addComment
 * @param grantService - TaskGrantService with resolvePermission
 */
export async function onCommitComplete(
  commitMsg: string,
  projectId: string,
  userId: string,
  taskService: Pick<TaskService, 'findByRef' | 'update' | 'addComment'>,
  grantService: Pick<TaskGrantService, 'resolvePermission'>
): Promise<void> {
  const refs = [...commitMsg.matchAll(TASK_REF_REGEX)].map(m => m[1])
  for (const ref of refs) {
    const task = await taskService.findByRef(ref).catch(() => null)
    if (!task || task.projectId !== projectId) continue

    const perm = await grantService.resolvePermission(userId, task.id)
    if (!perm || !EDIT_LEVEL_OR_HIGHER.has(perm)) continue

    await taskService.update(task.id, { status: 'review' })

    // Add activity comment when commit message contains "close(s)"
    if (/closes?\s+/i.test(commitMsg)) {
      await taskService.addComment(task.id, userId, `Commit: ${commitMsg}`, 'activity')
    }
  }
}
