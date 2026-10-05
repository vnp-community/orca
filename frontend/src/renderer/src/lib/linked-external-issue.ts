import type { CreateWorktreeArgs } from '../../../shared/types'
import { getLinkedWorkItemProvider } from './linked-work-item-provider'
import type { LinkedWorkItemSummary } from './new-workspace'

// Why: only Jira is forwarded. Linear/GitHub/GitLab links already travel on
// their own createWorktree fields, and recording them under this generic link
// too would start issue-status-sync transitions for providers whose behavior
// has not changed.
export function getLinkedExternalIssue(
  item: LinkedWorkItemSummary | null | undefined,
  taskProjectId?: string | null
): CreateWorktreeArgs['linkedExternalIssue'] {
  if (!item || getLinkedWorkItemProvider(item) !== 'jira') {
    return undefined
  }
  const ref = item.jiraIdentifier?.trim()
  if (!ref) {
    return undefined
  }
  return {
    provider: 'jira',
    ref,
    title: item.title,
    url: item.url,
    ...(taskProjectId ? { taskProjectId } : {}),
    ...(item.jiraSiteId ? { site: item.jiraSiteId } : {})
  }
}
