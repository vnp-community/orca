/**
 * open-review-entry.ts — FE-CV-TASK-061-02
 *
 * The single way every entry point (agent row, Source Control, Cmd+K, sidebar tab, notification)
 * opens Review. Flag off => no-op and no codeIntel.* traffic.
 */

import { useAppStore } from '@/store'
import { activateAndRevealWorktree } from '@/lib/worktree-activation'
import { ensureReviewTab } from '@/lib/ensure-review-tab'
import { setReviewOpenSource, takeReviewOpenSource } from '@/lib/review-open-source'
import type { ReviewOpenSource } from '@/lib/review-open-source'
import type { ReviewChipId } from '../review-chip-filter'
import { markTurnReviewed } from './reviewed-turn-memory'
import { selectAgentTurnCompletions } from './agent-turn-completion'

export type ReviewEntrySource =
  | 'agent-row'
  | 'source-control'
  | 'cmd-k'
  | 'right-sidebar'
  | 'notification'

export type OpenReviewEntryOptions = {
  lens?: string
  completionId?: string
  filter?: ReviewChipId
}

const TELEMETRY_SOURCE: Record<ReviewEntrySource, ReviewOpenSource> = {
  'agent-row': 'agent_row',
  'source-control': 'source_control',
  'cmd-k': 'cmd_k',
  'right-sidebar': 'right_sidebar',
  notification: 'notification'
}

export function openReviewFromEntryPoint(
  worktreeId: string,
  source: ReviewEntrySource,
  options?: OpenReviewEntryOptions
): boolean {
  const state = useAppStore.getState()
  if (state.codeIntelSupportState.state !== 'enabled') {
    return false
  }
  // Why: activate first so ensureReviewTab surfaces the tab in the now-active worktree.
  if (state.activeWorktreeId !== worktreeId && activateAndRevealWorktree(worktreeId) === false) {
    return false
  }
  const reviewedId =
    options?.completionId ??
    selectAgentTurnCompletions(useAppStore.getState(), worktreeId)[0]?.id ??
    null
  // Why: the workspace reads the origin once when it first shows data; set it before the tab mounts.
  setReviewOpenSource(worktreeId, {
    source: TELEMETRY_SOURCE[source],
    afterAgentTurn: reviewedId !== null
  })
  const tabId = ensureReviewTab(worktreeId)
  if (!tabId) {
    takeReviewOpenSource(worktreeId)
    return false
  }
  const store = useAppStore.getState()
  if (reviewedId) {
    markTurnReviewed(worktreeId, reviewedId)
  }
  // Scope stays untouched: the shell resolves the default (branch vs base) when it is null.
  if (options?.lens) {
    store.setReviewLens(worktreeId, options.lens)
  }
  if (options?.filter && store.reviewUiByWorktree[worktreeId]?.chipFilter !== options.filter) {
    store.toggleReviewChipFilter(worktreeId, options.filter)
  }
  return true
}
