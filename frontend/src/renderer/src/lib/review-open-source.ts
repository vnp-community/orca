/**
 * review-open-source.ts — FE-CV-TASK-095-06
 *
 * Entry points call `setReviewOpenSource` right before opening the Review tab; the workspace
 * takes it once when it first shows data. Without a caller the open counts as a restore
 * (tab brought back by session restore), which is the only path that opens without an entry.
 *
 * @module lib/review-open-source
 */

export type ReviewOpenSource =
  | 'agent_row'
  | 'source_control'
  | 'cmd_k'
  | 'right_sidebar'
  | 'notification'
  | 'restore'

export type ReviewOpenOrigin = { source: ReviewOpenSource; afterAgentTurn: boolean }

const pendingByWorktree = new Map<string, ReviewOpenOrigin>()

export function setReviewOpenSource(worktreeId: string, origin: ReviewOpenOrigin): void {
  pendingByWorktree.set(worktreeId, origin)
}

export function takeReviewOpenSource(worktreeId: string): ReviewOpenOrigin {
  const origin = pendingByWorktree.get(worktreeId) ?? { source: 'restore', afterAgentTurn: false }
  pendingByWorktree.delete(worktreeId)
  return origin
}
