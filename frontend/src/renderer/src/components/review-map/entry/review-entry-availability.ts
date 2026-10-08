/**
 * review-entry-availability.ts — FE-CV-TASK-061-02
 *
 * Pure availability rule for every Review entry point. Flag off/unknown/unsupported hides the
 * entry (no lockout UI, no codeIntel.* calls); 'unknown' stays hidden to avoid flicker.
 */

import type { CodeIntelSupportState } from '@/store/slices/code-intel'
import type { CodeIntelSelector } from '@/lib/code-intel-worktree-selector'

export type ReviewEntryUnavailableReason = 'not-enabled' | 'not-git' | 'no-active-worktree'

export type ReviewEntryAvailability =
  | { visible: true }
  | { visible: false; reason: ReviewEntryUnavailableReason }

export function resolveReviewEntryAvailability(
  worktreeId: string | null | undefined,
  supportState: Pick<CodeIntelSupportState, 'state'>,
  selector: CodeIntelSelector | null
): ReviewEntryAvailability {
  if (!worktreeId) {
    return { visible: false, reason: 'no-active-worktree' }
  }
  if (supportState.state !== 'enabled') {
    return { visible: false, reason: 'not-enabled' }
  }
  if (!selector || selector.state !== 'ready') {
    return { visible: false, reason: 'not-git' }
  }
  return { visible: true }
}
