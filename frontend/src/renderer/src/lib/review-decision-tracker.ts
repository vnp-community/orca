/**
 * review-decision-tracker.ts — FE-CV-TASK-095-04
 *
 * Tracks the "agent turn completed → user decision" lifecycle for telemetry.
 * State is in-memory only; not persisted across sessions.
 *
 * Lifecycle:
 *   1. registerCompletion(worktreeId) — agent turn finishes
 *   2. noteReviewOpened(worktreeId)   — user opens Review workspace
 *   3. decide(worktreeId, outcome)    — user commits, creates PR, etc.
 *
 * A new registerCompletion before decide() emits an "abandon" event for the
 * previous turn, then resets.
 *
 * @module lib/review-decision-tracker
 */

import {
  bucketDwellMs,
  bucketLarge,
  trackReviewQualityGateViewed,
} from './review-telemetry'

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export type ReviewDecisionOutcome =
  | 'commit'
  | 'pr'
  | 'no_action'
  | 'abandon'

export type GateResultForDecision = {
  overallRisk: string
  openFindings: number
}

type TrackerEntry = {
  worktreeFingerprint: string
  /** When the agent turn completed */
  completedAt: number
  /** When user opened Review (if they did) */
  reviewOpenedAt: number | null
  gateResult: GateResultForDecision | null
  /** Whether this entry has already emitted a decision event */
  decided: boolean
}

// ---------------------------------------------------------------------------
// Module-level tracker state (not persisted)
// ---------------------------------------------------------------------------

const entries = new Map<string, TrackerEntry>()

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

/**
 * Register that an agent turn completed for the given worktree.
 * If a prior entry exists and hasn't decided, emit an "abandon" event.
 */
export function registerCompletion(
  worktreeFingerprint: string,
  gateResult?: GateResultForDecision
): void {
  const existing = entries.get(worktreeFingerprint)
  if (existing && !existing.decided) {
    _emitDecision(existing, 'abandon')
  }

  entries.set(worktreeFingerprint, {
    worktreeFingerprint,
    completedAt: Date.now(),
    reviewOpenedAt: null,
    gateResult: gateResult ?? null,
    decided: false,
  })
}

/**
 * Record that the user opened the Review workspace after an agent turn.
 */
export function noteReviewOpened(worktreeFingerprint: string): void {
  const entry = entries.get(worktreeFingerprint)
  if (!entry || entry.decided) return
  if (!entry.reviewOpenedAt) {
    entry.reviewOpenedAt = Date.now()
  }
}

/**
 * Emit a decision event. Subsequent calls for the same entry are no-ops.
 */
export function decide(
  worktreeFingerprint: string,
  outcome: Exclude<ReviewDecisionOutcome, 'abandon'>
): void {
  const entry = entries.get(worktreeFingerprint)
  if (!entry || entry.decided) return
  _emitDecision(entry, outcome)
}

// ---------------------------------------------------------------------------
// Internal emit
// ---------------------------------------------------------------------------

function _emitDecision(
  entry: TrackerEntry,
  outcome: ReviewDecisionOutcome
): void {
  entry.decided = true

  const dwellMs = entry.reviewOpenedAt != null ? Date.now() - entry.reviewOpenedAt : 0
  const latencyMs = Date.now() - entry.completedAt

  // Emit gate viewed event (if gate data is available)
  if (entry.gateResult) {
    trackReviewQualityGateViewed({
      worktreeFingerprint: entry.worktreeFingerprint,
      overallRisk: entry.gateResult.overallRisk,
    })
  }

  // Decision telemetry: coarse payload only
  // NOTE: track() is fire-and-forget, never throws
  void Promise.resolve().then(() => {
    try {
      const { track } = require('./telemetry') as { track: (event: string, payload: Record<string, unknown>) => void }
      track('review_decision', {
        worktree_fingerprint: entry.worktreeFingerprint,
        outcome,
        used_review: entry.reviewOpenedAt != null,
        dwell_bucket: bucketDwellMs(dwellMs),
        latency_bucket: bucketDwellMs(latencyMs),
        open_findings: bucketLarge(entry.gateResult?.openFindings ?? 0),
        gate: entry.gateResult?.overallRisk ?? 'none',
      })
    } catch {
      // Telemetry failure is non-fatal
    }
  })
}

/**
 * Clear tracker state (for testing).
 */
export function _resetTrackerForTest(): void {
  entries.clear()
}
