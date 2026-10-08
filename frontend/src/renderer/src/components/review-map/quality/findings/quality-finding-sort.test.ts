import { describe, expect, it } from 'vitest'
import { makeQualityFinding as f } from '../../../../test-support/quality-finding-fixtures'
import { sortQualityFindings } from './quality-finding-sort'
import { countWaivedFindings, filterQualityFindings } from './quality-finding-filter'

describe('sortQualityFindings', () => {
  it('orders by severity, category, file, line, fingerprint and does not mutate', () => {
    const input = [
      f({ fingerprint: 'e', severity: 'info' }),
      f({ fingerprint: 'd', severity: 'unknown' }),
      f({ fingerprint: 'c', severity: 'error', category: 'test' }),
      f({ fingerprint: 'b', severity: 'error', category: 'lint', file: 'b.ts' }),
      f({ fingerprint: 'a2', severity: 'error', category: 'lint', file: 'a.ts', line: 9 }),
      f({ fingerprint: 'a1', severity: 'error', category: 'lint', file: 'a.ts', line: 2 }),
      f({ fingerprint: 'a0', severity: 'error', category: 'lint', file: 'a.ts', line: 2 })
    ]
    const copy = [...input]
    expect(sortQualityFindings(input).map((x) => x.fingerprint)).toEqual([
      'a0',
      'a1',
      'a2',
      'b',
      'c',
      'e',
      'd'
    ])
    expect(input).toEqual(copy)
  })

  it('is stable regardless of the input order', () => {
    const list = [f({ fingerprint: '3' }), f({ fingerprint: '1' }), f({ fingerprint: '2' })]
    expect(sortQualityFindings(list)).toEqual(sortQualityFindings(list.toReversed()))
  })
})

describe('filterQualityFindings', () => {
  const waiver = { by: 'u', reason: 'r', expiresAt: '2026-11-01T00:00:00Z' }
  const list = [
    f({ fingerprint: '1', ruleId: 'rule-x', message: 'Unused Variable' }),
    f({ fingerprint: '2', ruleId: 'TS2322', message: 'other' }),
    f({ fingerprint: '3', ruleId: 'rule-x', file: 'lib/Zed.ts', message: 'other', waiver })
  ]

  it('hides waived rows unless asked and counts them', () => {
    expect(
      filterQualityFindings(list, { search: '', showWaived: false }).map((x) => x.fingerprint)
    ).toEqual(['1', '2'])
    expect(filterQualityFindings(list, { search: '', showWaived: true })).toHaveLength(3)
    expect(countWaivedFindings(list)).toBe(1)
  })

  it('searches message, rule id and file case-insensitively', () => {
    const all = { showWaived: true }
    expect(
      filterQualityFindings(list, { ...all, search: 'unused' }).map((x) => x.fingerprint)
    ).toEqual(['1'])
    expect(
      filterQualityFindings(list, { ...all, search: 'ts2322' }).map((x) => x.fingerprint)
    ).toEqual(['2'])
    expect(
      filterQualityFindings(list, { ...all, search: 'ZED' }).map((x) => x.fingerprint)
    ).toEqual(['3'])
  })
})
