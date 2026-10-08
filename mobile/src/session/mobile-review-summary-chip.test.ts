import { describe, expect, it } from 'vitest'
import {
  buildMobileReviewSummaryChip,
  buildMobileReviewSummaryRoute
} from './mobile-review-summary-chip'
import type { MobileReviewSummary } from './mobile-review-summary-rpc'

function ready(over: Partial<MobileReviewSummary> = {}) {
  return {
    kind: 'ready' as const,
    summary: {
      available: true,
      risk: { level: 'LOW' as const, reasons: [] },
      findings: {
        totalOpen: 0,
        bySeverity: { error: 0, warning: 0, info: 0 },
        items: [],
        truncated: false
      },
      ...over
    }
  }
}

describe('buildMobileReviewSummaryRoute', () => {
  it('encodes host, worktree and the optional name', () => {
    expect(buildMobileReviewSummaryRoute({ hostId: 'h 1', worktreeId: 'repo::/tmp/wt' })).toBe(
      '/h/h%201/review-summary/repo%3A%3A%2Ftmp%2Fwt'
    )
    expect(buildMobileReviewSummaryRoute({ hostId: 'h', worktreeId: 'w', name: 'feat x' })).toBe(
      '/h/h/review-summary/w?name=feat+x'
    )
  })
})

describe('buildMobileReviewSummaryChip', () => {
  it('shows nothing until the summary is known to be available', () => {
    expect(buildMobileReviewSummaryChip({ kind: 'loading' })).toBeNull()
    expect(buildMobileReviewSummaryChip({ kind: 'unavailable', message: 'x' })).toBeNull()
    expect(buildMobileReviewSummaryChip({ kind: 'error', message: 'x' })).toBeNull()
  })

  it('is neutral for a clean low-risk summary', () => {
    expect(buildMobileReviewSummaryChip(ready())).toEqual({
      label: 'Review summary',
      detail: 'Risk Low · 0 open findings',
      tone: 'neutral'
    })
  })

  it('escalates tone by errors, warnings and risk level, and flags a stale index', () => {
    const withErrors = ready({
      findings: {
        totalOpen: 1,
        bySeverity: { error: 1, warning: 0, info: 0 },
        items: [],
        truncated: false
      }
    })
    expect(buildMobileReviewSummaryChip(withErrors)).toMatchObject({
      tone: 'danger',
      detail: 'Risk Low · 1 open finding'
    })
    expect(
      buildMobileReviewSummaryChip(ready({ risk: { level: 'MEDIUM', reasons: [] } }))?.tone
    ).toBe('warning')
    expect(
      buildMobileReviewSummaryChip(ready({ risk: { level: 'CRITICAL', reasons: [] }, stale: true }))
    ).toMatchObject({ tone: 'danger', detail: 'Risk Critical · 0 open findings · index stale' })
    expect(
      buildMobileReviewSummaryChip(ready({ risk: undefined, findings: undefined }))?.detail
    ).toBe('Risk Unknown · 0 open findings')
  })
})
