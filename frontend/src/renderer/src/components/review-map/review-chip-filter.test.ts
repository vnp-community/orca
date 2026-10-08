import { describe, expect, it } from 'vitest'
import {
  isFilteringChip,
  REVIEW_CHIP_IDS,
  reviewChipCount,
  reviewChipPredicate
} from './review-chip-filter'
import { makeOverlay } from './review-test-data'

const sym = (key: string, filePath: string) => ({ key, kind: 'function', name: key, filePath })

describe('reviewChipCount', () => {
  it('uses array length when not truncated', () => {
    expect(reviewChipCount(makeOverlay(), 'files')).toBe(3)
  })
  it('uses totalCounts when the array was truncated', () => {
    const o = makeOverlay({
      limits: { truncated: { files: true }, totalCounts: { changedFiles: 250 } }
    })
    expect(reviewChipCount(o, 'files')).toBe(250)
  })
  it('ignores a missing totalCounts key', () => {
    const o = makeOverlay({ limits: { truncated: { files: true }, totalCounts: {} } })
    expect(reviewChipCount(o, 'files')).toBe(3)
  })
  it('seven chips exist', () => {
    expect(REVIEW_CHIP_IDS).toHaveLength(7)
  })
})

describe('reviewChipPredicate', () => {
  it('files/symbols/untested/violations return key sets', () => {
    const o = makeOverlay({
      changedSymbols: [{ symbol: sym('a', 'x.ts'), changeKind: 'modified', tested: 'yes' }],
      uncoveredSymbols: [sym('b', 'y.ts')],
      violations: [
        { findingKey: 'k', rule: 'r', severity: 'error', file: 'z.ts', status: 'touched' }
      ]
    })
    expect([...reviewChipPredicate(o, 'files')!.files]).toEqual([
      'src/f1.ts',
      'src/f2.ts',
      'src/f3.ts'
    ])
    expect(reviewChipPredicate(o, 'symbols')!.symbolKeys).toEqual(new Set(['a']))
    expect([...reviewChipPredicate(o, 'untested')!.files]).toEqual(['y.ts'])
    expect([...reviewChipPredicate(o, 'violations')!.files]).toEqual(['z.ts'])
  })
  it('navigation chips do not filter', () => {
    for (const c of ['flows', 'tables', 'contracts'] as const) {
      expect(isFilteringChip(c)).toBe(false)
      expect(reviewChipPredicate(makeOverlay(), c)).toBeNull()
    }
  })
})
