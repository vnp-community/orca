import { beforeEach, describe, expect, it, vi } from 'vitest'

const track = vi.hoisted(() => vi.fn())
vi.mock('@/lib/telemetry', () => ({ track }))

import { eventSchemas } from '../../../../../shared/telemetry-events'
import { trackAiSummaryFeedback } from './ai-summary-feedback'
import type { UseReviewAiSummaryResult } from './use-review-ai-summary'

function ai(over: Partial<UseReviewAiSummaryResult> = {}): UseReviewAiSummaryResult {
  return {
    state: 'ready',
    level: 'diff',
    allowedLevels: ['metadata', 'diff'],
    manifest: null,
    view: { summary: null, manifest: null, cache: { hit: true, createdAt: null, expiresAt: null } },
    error: null,
    retryAfterSeconds: null,
    preview: async () => {},
    confirmAndGenerate: async () => {},
    cancel: () => {},
    ...over
  }
}

beforeEach(() => track.mockClear())

describe('trackAiSummaryFeedback', () => {
  it('sends an enum-only review_ai_summary event that passes the schema', () => {
    expect(trackAiSummaryFeedback(ai(), 'incorrect')).toBe(true)
    expect(track).toHaveBeenCalledWith('review_ai_summary', {
      outcome: 'ok',
      level: 'diff',
      cache_hit: true,
      feedback: 'wrong'
    })
    expect(eventSchemas.review_ai_summary.safeParse(track.mock.calls[0][1]).success).toBe(true)
  })

  it('maps helpful to useful and sends only once per summary', () => {
    const state = ai()
    expect(trackAiSummaryFeedback(state, 'helpful')).toBe(true)
    expect(trackAiSummaryFeedback(state, 'helpful')).toBe(false)
    expect(track).toHaveBeenCalledTimes(1)
    expect(track.mock.calls[0][1].feedback).toBe('useful')
  })

  it('does nothing when no summary is ready', () => {
    expect(trackAiSummaryFeedback(ai({ state: 'generating' }), 'helpful')).toBe(false)
    expect(trackAiSummaryFeedback(ai({ view: null }), 'helpful')).toBe(false)
    expect(track).not.toHaveBeenCalled()
  })
})
