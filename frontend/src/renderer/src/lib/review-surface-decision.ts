/**
 * review-surface-decision.ts — FE-CV-TASK-095-05
 *
 * Records `send_to_agent` / `mark_reviewed` (Review workspace) and `create_review` (Checks panel)
 * into the decision tracker.
 * Reads flags lazily from the store (no second turn subscription: the Source Control hook already
 * registers completions, and registering twice would report the first as abandoned).
 *
 * @module lib/review-surface-decision
 */

import { useAppStore } from '@/store'
import { decide } from './review-decision-tracker'
import type { ReviewDecision } from './review-decision-tracker'

export function recordReviewSurfaceDecision(
  worktreeId: string,
  decision: Extract<ReviewDecision, 'send_to_agent' | 'mark_reviewed' | 'create_review'>
): void {
  try {
    const support = useAppStore.getState().codeIntelSupportState
    if (support.state !== 'enabled') {
      return
    }
    const quality = support.effective?.qualityGateEnabled === true
    // The gate verdict lives with Source Control; from here it is unknown when the gate is on.
    decide(worktreeId, decision, { gate: quality ? 'unknown' : 'none', openFindings: 0 })
  } catch {
    // Telemetry must never break sending notes or reading progress.
  }
}
