import { describe, expect, it } from 'vitest'
import { buildReviewSummaryModel } from './review-summary-model'
import type { ChangeOverlayView } from '../review-wire-types'

function overlay(over: Partial<ChangeOverlayView> = {}): ChangeOverlayView {
  return {
    scope: { baseRef: 'main', mode: 'worktree', includesUncommitted: true },
    changedFiles: [],
    changedSymbols: [],
    affectedFlows: [],
    touchedTables: [],
    touchedContracts: [],
    uncoveredSymbols: [],
    violations: [],
    readingOrder: [],
    components: [],
    risk: null,
    indexFreshness: { state: 'fresh' },
    limits: { truncated: {}, totalCounts: {} },
    ...over
  }
}

describe('buildReviewSummaryModel', () => {
  it('reads totalCounts and hides chips with unknown keys', () => {
    const m = buildReviewSummaryModel(
      overlay({ limits: { truncated: {}, totalCounts: { files: 12, symbols: 38, tables: 0 } } }),
      null
    )
    expect(m.counts).toEqual([
      { id: 'files', value: 12 },
      { id: 'symbols', value: 38 },
      { id: 'tables', value: 0 }
    ])
    expect(m.baseRef).toBe('main')
    expect(m.freshnessState).toBe('fresh')
  })

  it('falls back to array lengths only when non-empty', () => {
    const m = buildReviewSummaryModel(overlay({ affectedFlows: [{}, {}] }), null)
    expect(m.counts).toEqual([{ id: 'flows', value: 2 }])
  })

  it('counts findings by severity and ignores unknown severities', () => {
    const v = (severity: string) => ({
      findingKey: severity,
      rule: 'r',
      severity,
      file: 'a',
      status: 'touched'
    })
    const m = buildReviewSummaryModel(
      overlay({ violations: [v('error'), v('warning'), v('warning'), v('weird')] }),
      null
    )
    expect(m.findings).toEqual({ error: 1, warning: 2, info: 0 })
  })

  it('computes progress against the file total', () => {
    const progress = {
      version: 1 as const,
      lastFocusedKey: null,
      entries: { a: { state: 'seen' as const, at: 1 }, b: { state: 'unseen' as const, at: 1 } }
    }
    const withTotal = overlay({ limits: { truncated: {}, totalCounts: { files: 12 } } })
    expect(buildReviewSummaryModel(withTotal, progress).progress).toEqual({ seen: 1, total: 12 })
    expect(buildReviewSummaryModel(overlay(), progress).progress).toBeNull()
    expect(buildReviewSummaryModel(withTotal, null).progress).toBeNull()
  })
})
