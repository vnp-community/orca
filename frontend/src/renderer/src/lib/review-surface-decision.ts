/**
 * review-surface-decision.ts — FE-CV-TASK-095-05
 *
 * Records `send_to_agent` / `mark_reviewed` (Review workspace) and `create_review` (Checks panel)
 * into the decision tracker.
 * Gate verdict and open findings come from the cached quality gate in the store.
 * Reads flags lazily from the store (no second turn subscription: the Source Control hook already
 * registers completions, and registering twice would report the first as abandoned).
 *
 * @module lib/review-surface-decision
 */

import { useAppStore } from '@/store'
import { decide } from './review-decision-tracker'
import type { ReviewDecision, ReviewDecisionContext } from './review-decision-tracker'

/** Open findings = gate reasons still failing or warning (passed checks are not open). */
export function countOpenGateFindings(
  reasons: readonly { result?: unknown }[] | null | undefined
): number {
  return (reasons ?? []).filter((r) => r.result === 'fail' || r.result === 'warn').length
}

type CachedGate =
  | { verdict?: unknown; result?: unknown; reasons?: { result?: unknown }[] }
  | null
  | undefined

/** Decision context from the cached quality gate (never fetched here: telemetry adds no RPC). */
export function reviewDecisionContextFromGate(
  qualityEnabled: boolean,
  gate: CachedGate
): ReviewDecisionContext {
  if (!qualityEnabled) {
    return { gate: 'none', openFindings: 0 }
  }
  // The contract names the field `verdict`; older payloads used `result`.
  const verdict = gate ? (gate.verdict ?? gate.result) : undefined
  return {
    gate: verdict === 'pass' || verdict === 'warn' || verdict === 'fail' ? verdict : 'unknown',
    openFindings: countOpenGateFindings(gate?.reasons)
  }
}

export function recordReviewSurfaceDecision(
  worktreeId: string,
  decision: Extract<ReviewDecision, 'send_to_agent' | 'mark_reviewed' | 'create_review'>
): void {
  try {
    const state = useAppStore.getState()
    const support = state.codeIntelSupportState
    if (support.state !== 'enabled') {
      return
    }
    const quality = support.effective?.qualityGateEnabled === true
    const cached = state.codeIntelQualityByWorktree?.[worktreeId]?.gate?.data.gate as CachedGate
    decide(worktreeId, decision, reviewDecisionContextFromGate(quality, cached))
  } catch {
    // Telemetry must never break sending notes or reading progress.
  }
}
