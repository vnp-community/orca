/**
 * review-decision-tracker.ts — FE-CV-TASK-095-04
 *
 * Measures "agent finished -> what the user decided", one `review_decision_made` per
 * turn. In memory only (lost on quit, which is acceptable): a new completion for the
 * same worktree replaces the previous undecided one and reports it as `abandon`.
 *
 * @module lib/review-decision-tracker
 */

import { trackReviewDecisionMade } from './review-telemetry'

export type ReviewDecision = 'commit' | 'create_review' | 'send_to_agent' | 'mark_reviewed'
export type ReviewDecisionGate = 'pass' | 'warn' | 'fail' | 'unknown' | 'none'
export type ReviewDecisionContext = { gate: ReviewDecisionGate; openFindings: number }

type PendingTurn = { doneAt: number; reviewOpenedAt: number | null }

const pending = new Map<string, PendingTurn>()

export function registerCompletion(worktreeId: string, doneAt: number = Date.now()): void {
  const previous = pending.get(worktreeId)
  if (previous) {
    emit(previous, 'abandon', { gate: 'none', openFindings: 0 }, doneAt)
  }
  pending.set(worktreeId, { doneAt, reviewOpenedAt: null })
}

export function noteReviewOpened(worktreeId: string, at: number = Date.now()): void {
  const turn = pending.get(worktreeId)
  // Why: only the first open after the turn counts as "used review".
  if (turn && turn.reviewOpenedAt === null) {
    turn.reviewOpenedAt = at
  }
}

/** Emits exactly one decision for the pending turn; later calls (no pending turn) are ignored. */
export function decide(
  worktreeId: string,
  decision: ReviewDecision,
  context: ReviewDecisionContext,
  at: number = Date.now()
): void {
  const turn = pending.get(worktreeId)
  if (!turn) {
    return
  }
  pending.delete(worktreeId)
  emit(turn, decision, context, at)
}

function emit(
  turn: PendingTurn,
  decision: ReviewDecision | 'abandon',
  context: ReviewDecisionContext,
  at: number
): void {
  trackReviewDecisionMade({
    decision,
    latencyMs: at - turn.doneAt,
    usedReview: turn.reviewOpenedAt !== null,
    gate: context.gate,
    openFindings: context.openFindings
  })
}

export function hasPendingTurn(worktreeId: string): boolean {
  return pending.has(worktreeId)
}

/** Test seam: clears all pending turns without emitting. */
export function resetReviewDecisionTracker(): void {
  pending.clear()
}
