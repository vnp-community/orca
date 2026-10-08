/**
 * ai-summary-feedback.ts — FE-CV-TASK-093-04
 *
 * Sends the enum-only "helpful / incorrect" feedback for an AI summary as a `review_ai_summary`
 * event. No summary text or file names leave the client.
 *
 * @module components/review-map/ai-summary/ai-summary-feedback
 */

import { trackReviewAiSummary } from '@/lib/review-telemetry'
import type { UseReviewAiSummaryResult } from './use-review-ai-summary'

export type AiSummaryFeedbackValue = 'helpful' | 'incorrect'

// Why: one feedback per generated summary; repeated clicks must not inflate the counts.
const sentFor = new WeakSet<object>()

/** Returns false when nothing was sent (no ready summary, or feedback already given for it). */
export function trackAiSummaryFeedback(
  ai: UseReviewAiSummaryResult,
  value: AiSummaryFeedbackValue
): boolean {
  const view = ai.view
  if (ai.state !== 'ready' || !view || sentFor.has(view)) {
    return false
  }
  sentFor.add(view)
  trackReviewAiSummary({
    outcome: 'ok',
    level: ai.level,
    cache_hit: view.cache?.hit === true,
    feedback: value === 'helpful' ? 'useful' : 'wrong'
  })
  return true
}
