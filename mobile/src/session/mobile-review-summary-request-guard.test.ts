import { describe, expect, it } from 'vitest'
import { createMobileReviewSummaryRequestGuard } from './mobile-review-summary-request-guard'

describe('createMobileReviewSummaryRequestGuard', () => {
  it('only the latest request is current', () => {
    const guard = createMobileReviewSummaryRequestGuard()
    const first = guard.begin()
    expect(first()).toBe(true)
    const second = guard.begin()
    expect(first()).toBe(false)
    expect(second()).toBe(true)
  })
})
