/**
 * ai-summary-consent-state.ts — FE-CV-TASK-093-02
 *
 * Session-scoped consent tracking for AI summary generation.
 * No persistence — resets on reload.
 *
 * Per spec: consent is once per session AND once per level.
 *
 * @module components/review-map/ai-summary/ai-summary-consent-state
 */

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export type AiSummaryLevel = 'brief' | 'standard' | 'detailed'

// ---------------------------------------------------------------------------
// Session-scoped state
// ---------------------------------------------------------------------------

const confirmedLevels = new Set<AiSummaryLevel>()

export function hasConsentForLevel(level: AiSummaryLevel): boolean {
  return confirmedLevels.has(level)
}

export function recordConsentForLevel(level: AiSummaryLevel): void {
  confirmedLevels.add(level)
}

/** For testing only */
export function _resetConsentForTest(): void {
  confirmedLevels.clear()
}
