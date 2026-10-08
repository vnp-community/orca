import { describe, expect, it } from 'vitest'
import { buildCoverageReport } from '../../../test-support/code-intel-quality-visualization-fake-data'
import { buildCoverageTreemapItems, uncoveredPercent } from './quality-coverage-treemap-items'
import { listUncoveredFiles } from './quality-coverage-uncovered-files'

const file = (path: string, stmts: number, covered: number, extra: object = {}) => ({
  path,
  stmts,
  covered,
  pct: 0,
  ...extra
})

describe('buildCoverageTreemapItems', () => {
  it('sizes by statements and shades by the uncovered percent', () => {
    const model = buildCoverageTreemapItems([file('a.ts', 40, 30), file('b.ts', 10, 10)])
    expect(model.items).toEqual([
      { id: 'a.ts', label: 'a.ts', size: 40, intensity: 25 },
      { id: 'b.ts', label: 'b.ts', size: 10, intensity: 0 }
    ])
  })

  it('drops files without statements and counts them', () => {
    const model = buildCoverageTreemapItems([
      file('empty.ts', 0, 0),
      file('nan.ts', Number.NaN, 0),
      file('ok.ts', 5, 1)
    ])
    expect(model.items.map((i) => i.id)).toEqual(['ok.ts'])
    expect(model.withoutStatements).toBe(2)
  })

  it('cuts 1000 files to 400 tiles and says how many are hidden', () => {
    const files = Array.from({ length: 1000 }, (_, i) => file(`f-${i}.ts`, 1 + i, 0))
    const model = buildCoverageTreemapItems(files)
    expect(model.items).toHaveLength(400)
    expect(model.hidden).toBe(600)
    expect(model.drawable).toBe(1000)
    expect(model.items[0].id).toBe('f-999.ts')
  })

  it('caps covered above statements so intensity never goes negative', () => {
    expect(uncoveredPercent({ stmts: 10, covered: 12 })).toBe(0)
  })
})

describe('listUncoveredFiles', () => {
  it('orders by uncovered lines and ignores malformed ranges', () => {
    const { shown, total } = listUncoveredFiles([
      file('a.ts', 9, 0, { uncoveredRanges: [[1, 2]] }),
      file('b.ts', 9, 0, {
        uncoveredRanges: [
          [5, 9],
          [0, 3],
          [8, 2]
        ]
      }),
      file('c.ts', 9, 0)
    ])
    expect(total).toBe(2)
    expect(shown.map((f) => [f.path, f.lines])).toEqual([
      ['b.ts', 5],
      ['a.ts', 2]
    ])
  })

  it('shows at most 20 files but reports the total', () => {
    const files = Array.from({ length: 30 }, (_, i) =>
      file(`f${i}.ts`, 9, 0, { uncoveredRanges: [[1, i + 1]] })
    )
    const { shown, total } = listUncoveredFiles(files)
    expect(shown).toHaveLength(20)
    expect(total).toBe(30)
  })

  it('reads the fixture report', () => {
    expect(listUncoveredFiles(buildCoverageReport().files).total).toBe(6)
  })
})
