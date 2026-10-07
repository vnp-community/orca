/**
 * ensure-review-tab.ts — FE-CV-TASK-050-16
 *
 * Ensures one review tab per worktree; follows the ensureSimulatorTab pattern.
 * Focuses existing tab instead of creating duplicates.
 *
 * Returns null when:
 * - No target group found
 * - code-intel support is 'disabled' or 'unsupported'
 * - Selector is 'unsupported'
 * Does NOT return null when support is 'unknown' (allow open during probe).
 *
 * @module lib/ensure-review-tab
 */

import { useAppStore } from '@/store'
import { findReusableRightSplitGroupId } from './emulator-right-split-target'
import { translate } from '@/i18n/i18n'
import type { CodeIntelSupportState } from '../store/slices/code-intel'

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

type EnsureReviewTabOptions = {
  targetGroupId?: string
  placement?: 'activeGroup' | 'rightSplit'
  /** When true, activate the tab and focus the owning group (default true). */
  surfacePane?: boolean
}

type ExistingReviewTab = {
  id: string
  groupId: string
  contentType: string
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

export function getReviewTabForWorktree(worktreeId: string): ExistingReviewTab | null {
  return (
    (useAppStore.getState().unifiedTabsByWorktree[worktreeId] ?? []).find(
      (tab) => tab.contentType === 'review'
    ) ?? null
  )
}

function getSupportState(): CodeIntelSupportState['state'] {
  const state = useAppStore.getState() as Record<string, unknown>
  const support = state.codeIntelSupportState as CodeIntelSupportState | undefined
  return support?.state ?? 'unknown'
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

/** One review tab per worktree; focuses existing tab instead of creating duplicates. */
export function ensureReviewTab(
  worktreeId: string,
  options?: EnsureReviewTabOptions
): string | null {
  const store = useAppStore.getState()

  // Block when support is explicitly off
  const supportState = getSupportState()
  if (supportState === 'disabled' || supportState === 'unsupported') {
    return null
  }
  // 'unknown' → allow open (probe in progress)

  const sourceGroupId =
    options?.targetGroupId ??
    store.activeGroupIdByWorktree[worktreeId] ??
    store.groupsByWorktree[worktreeId]?.[0]?.id

  if (!sourceGroupId) {
    return null
  }

  const existing = getReviewTabForWorktree(worktreeId)
  const shouldSurface = options?.surfacePane ?? true

  if (existing) {
    if (shouldSurface && store.activeWorktreeId === worktreeId) {
      store.activateTab(existing.id)
      store.focusGroup(worktreeId, existing.groupId)
      store.setActiveTabType('review')
    }
    return existing.id
  }

  const label = translate(
    'auto.lib.ensure.review.tab.title',
    'Review'
  )

  if (options?.placement === 'rightSplit' && shouldSurface) {
    const reusableRightGroupId = findReusableRightSplitGroupId(
      store.layoutByWorktree[worktreeId],
      sourceGroupId
    )
    if (reusableRightGroupId) {
      const tab = store.createUnifiedTab(worktreeId, 'review', {
        label,
        targetGroupId: reusableRightGroupId,
        activate: true
      })
      store.activateTab(tab.id)
      store.setActiveTabType('review')
      store.focusGroup(worktreeId, tab.groupId)
      return tab.id
    }

    const splitTab = store.createUnifiedTabInSplit(
      worktreeId,
      'review',
      { sourceGroupId, splitDirection: 'right' },
      { label, activate: true }
    )
    if (splitTab) {
      return splitTab.id
    }
  }

  const tab = store.createUnifiedTab(worktreeId, 'review', {
    label,
    targetGroupId: sourceGroupId,
    activate: shouldSurface
  })

  if (shouldSurface) {
    store.activateTab(tab.id)
    store.setActiveTabType('review')
    store.focusGroup(worktreeId, tab.groupId)
  }

  return tab.id
}
