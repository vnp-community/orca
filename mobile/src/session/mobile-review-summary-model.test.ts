import { describe, expect, it } from 'vitest'
import {
  canOpenMobileReviewFindingDiff,
  filterMobileReviewFindings,
  formatMobileReviewAge,
  mobileReviewRiskLabel,
  mobileReviewTruncationLabel,
  sortMobileReviewFindings
} from './mobile-review-summary-model'
import type { MobileReviewSummaryFinding } from './mobile-review-summary-rpc'

function f(
  key: string,
  severity: MobileReviewSummaryFinding['severity'],
  origin: MobileReviewSummaryFinding['origin'],
  extra: Partial<MobileReviewSummaryFinding> = {}
): MobileReviewSummaryFinding {
  return {
    key,
    kind: 'k',
    severity,
    title: key,
    summary: '',
    origin,
    inChangedFiles: false,
    ...extra
  }
}

const items = [
  f('a', 'info', 'introduced'),
  f('b', 'error', 'preexisting'),
  f('c', 'error', 'introduced'),
  f('d', 'warning', 'introduced')
]

describe('mobile review summary model', () => {
  it('filters by severity and by origin', () => {
    expect(filterMobileReviewFindings(items, 'all')).toHaveLength(4)
    expect(filterMobileReviewFindings(items, 'error').map((i) => i.key)).toEqual(['b', 'c'])
    expect(filterMobileReviewFindings(items, 'warning').map((i) => i.key)).toEqual(['d'])
    expect(filterMobileReviewFindings(items, 'introduced').map((i) => i.key)).toEqual([
      'a',
      'c',
      'd'
    ])
  })

  it('sorts by severity then origin without mutating input', () => {
    expect(sortMobileReviewFindings(items).map((i) => i.key)).toEqual(['c', 'b', 'd', 'a'])
    expect(items[0]?.key).toBe('a')
  })

  it('formats age', () => {
    const now = Date.parse('2026-10-07T12:00:00Z')
    expect(formatMobileReviewAge('2026-10-07T11:58:00Z', now)).toBe('2 min ago')
    expect(formatMobileReviewAge('2026-10-07T11:59:50Z', now)).toBe('just now')
    expect(formatMobileReviewAge('2026-10-07T09:00:00Z', now)).toBe('3 h ago')
    expect(formatMobileReviewAge('bad', now)).toBeNull()
    expect(formatMobileReviewAge(undefined, now)).toBeNull()
  })

  it('labels risk, truncation and diff eligibility', () => {
    expect(mobileReviewRiskLabel('MEDIUM')).toBe('Medium')
    expect(mobileReviewRiskLabel('UNKNOWN')).toBe('Unknown')
    expect(
      mobileReviewTruncationLabel({
        available: true,
        findings: {
          totalOpen: 80,
          bySeverity: { error: 0, warning: 0, info: 0 },
          items,
          truncated: true
        }
      })
    ).toBe('Showing 4 / 80')
    expect(mobileReviewTruncationLabel({ available: true })).toBeNull()
    expect(
      canOpenMobileReviewFindingDiff(
        f('x', 'info', 'unknown', { inChangedFiles: true, filePath: 'a.ts' })
      )
    ).toBe(true)
    expect(canOpenMobileReviewFindingDiff(f('x', 'info', 'unknown', { filePath: 'a.ts' }))).toBe(
      false
    )
  })
})
