/**
 * checks-panel-review-decision.ts — FE-CV-TASK-095-05
 *
 * The Checks panel's `create_review` decision, kept out of ChecksPanel.tsx so the call site is
 * testable without rendering the whole panel.
 *
 * @module components/right-sidebar/checks-panel-review-decision
 */

import { recordReviewSurfaceDecision } from '@/lib/review-surface-decision'

/** Records `create_review` only for a successful create in a known worktree. */
export function recordChecksPanelReviewCreated(
  worktreeId: string | null | undefined,
  result: { ok: boolean }
): void {
  if (!result.ok || !worktreeId) {
    return
  }
  // CR-095: the PR created here is the turn's decision when no Source Control path took it first.
  recordReviewSurfaceDecision(worktreeId, 'create_review')
}
