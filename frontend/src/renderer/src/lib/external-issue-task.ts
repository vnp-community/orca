import type { CreateWorktreeArgs } from '../../../shared/types'

type RuntimeCall = (method: string, params: Record<string, unknown>) => Promise<unknown>

// Best-effort by design: the workspace already exists and is the primary
// outcome, so a failure here must never surface as a failed "Start work".
// task.createFromSource is idempotent per (project, provider, ref), so a
// second start on the same issue returns the existing task.
export async function createTaskForExternalIssue(
  call: RuntimeCall,
  issue: NonNullable<CreateWorktreeArgs['linkedExternalIssue']>
): Promise<boolean> {
  if (!issue.taskProjectId) {
    return false
  }
  try {
    await call('task.createFromSource', {
      title: issue.title?.trim() || issue.ref,
      projectId: issue.taskProjectId,
      provider: issue.provider,
      ref: issue.ref,
      url: issue.url ?? '',
      ...(issue.site ? { site: issue.site } : {})
    })
    return true
  } catch (error) {
    console.warn('[task-link] could not create a task for the linked issue', error)
    return false
  }
}
