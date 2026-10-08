import { useAppStore } from '@/store'
import { useCodeIntelSelector } from '@/lib/code-intel-worktree-selector'
import {
  resolveReviewEntryAvailability,
  type ReviewEntryAvailability
} from './review-entry-availability'

// Why: partial store mocks and early boot may lack the slice; treat as unknown (hidden).
const UNKNOWN_SUPPORT = { state: 'unknown' } as const
const NOT_VISIBLE_NO_WORKTREE: ReviewEntryAvailability = {
  visible: false,
  reason: 'no-active-worktree'
}
const VISIBLE: ReviewEntryAvailability = { visible: true }
const NOT_ENABLED: ReviewEntryAvailability = { visible: false, reason: 'not-enabled' }
const NOT_GIT: ReviewEntryAvailability = { visible: false, reason: 'not-git' }

export function useReviewEntryAvailability(
  worktreeId: string | null | undefined
): ReviewEntryAvailability {
  const supportState = useAppStore((s) => s.codeIntelSupportState) ?? UNKNOWN_SUPPORT
  const selector = useCodeIntelSelector(worktreeId ?? '')
  if (!worktreeId) {
    return NOT_VISIBLE_NO_WORKTREE
  }
  const result = resolveReviewEntryAvailability(worktreeId, supportState, selector)
  if (result.visible) {
    return VISIBLE
  }
  return result.reason === 'not-enabled' ? NOT_ENABLED : NOT_GIT
}
