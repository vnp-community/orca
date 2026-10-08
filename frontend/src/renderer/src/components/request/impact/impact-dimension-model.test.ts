import { describe, expect, it } from 'vitest'
import { IMPACT_DIMENSIONS, buildComparisonRows, sortByRiskDesc, summarizeCard } from './impact-dimension-model'
import type { ImpactComparison, ImpactSummary } from '../../../../../shared/request-artifact-types'

const cmp = (optionId: string, dimensions: ImpactComparison['dimensions']): ImpactComparison => ({ optionId, dimensions })

describe('buildComparisonRows', () => {
  it('returns no rows without comparison data', () => {
    expect(buildComparisonRows(['a', 'b'], null)).toEqual([])
    expect(buildComparisonRows(['a'], [])).toEqual([])
  })

  it('lists the nine dimensions, marks differing rows and leaves missing cells null', () => {
    const rows = buildComparisonRows(['a', 'b'], [
      cmp('a', { data: { level: 'high', score: 3 }, security: { level: 'low', score: 1 } }),
      cmp('b', { data: { level: 'low', score: 1 }, security: { level: 'low', score: 1 } })
    ])
    expect(rows.map((r) => r.dimension).slice(0, 9)).toEqual([...IMPACT_DIMENSIONS])
    expect(rows.find((r) => r.dimension === 'data')?.differs).toBe(true)
    expect(rows.find((r) => r.dimension === 'security')?.differs).toBe(false)
    expect(rows.find((r) => r.dimension === 'size')?.cells).toEqual({ a: null, b: null })
  })

  it('appends unknown backend dimensions', () => {
    const rows = buildComparisonRows(['a'], [cmp('a', { latency: { level: 'medium', score: null } })])
    expect(rows.at(-1)?.dimension).toBe('latency')
  })
})

describe('summarizeCard', () => {
  const base: ImpactSummary = {
    assessmentId: 'a', digest: 'd', level: 'unknown', score: null, topReasons: ['1', '2', '3', '4'], confidence: null,
    assessedAt: null, tool: null, stale: false, mode: 'shadow', status: 'ready', hardRules: []
  }
  it('returns null for no summary and never invents a level', () => {
    expect(summarizeCard(null)).toBeNull()
    expect(summarizeCard(base)?.level).toBe('unknown')
  })
  it('formats score, caps reasons, flags shadow mode', () => {
    const s = summarizeCard({ ...base, level: 'high', score: 61.6, mode: 'enforce' })
    expect(s).toMatchObject({ scoreText: '62/100', advisory: false })
    expect(s?.reasons).toHaveLength(3)
    expect(summarizeCard(base)?.advisory).toBe(true)
  })
  it('sorts by descending risk', () => {
    expect(sortByRiskDesc([{ level: 'low' as const }, { level: 'critical' as const }, { level: 'unknown' as const }]).map((x) => x.level)).toEqual(['critical', 'low', 'unknown'])
  })
})
