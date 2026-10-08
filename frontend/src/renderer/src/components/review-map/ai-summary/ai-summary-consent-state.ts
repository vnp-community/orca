/**
 * ai-summary-consent-state.ts — FE-CV-TASK-093-02
 *
 * Which data levels the user already agreed to send to the LLM provider in this
 * session. In memory only: a reload asks again, and so does switching level.
 *
 * @module components/review-map/ai-summary/ai-summary-consent-state
 */

import type { AiSummaryLevel } from './ai-summary-wire-parser'

const confirmedLevels = new Set<AiSummaryLevel>()

export function hasConsentForLevel(level: AiSummaryLevel): boolean {
  return confirmedLevels.has(level)
}

export function recordConsentForLevel(level: AiSummaryLevel): void {
  confirmedLevels.add(level)
}

export function resetAiSummaryConsent(): void {
  confirmedLevels.clear()
}
