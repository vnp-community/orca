import { describe, expect, it } from 'vitest'
import type { Finding } from '../../../../../shared/code-intel-findings-types'
import { buildHotspotFindings } from '../../../test-support/code-intel-quality-visualization-fake-data'
import { buildHotspotModel } from './quality-hotspot-rows'

const hot = (
  path: string,
  metrics: Record<string, number>,
  extra: Partial<Finding> = {}
): Finding =>
  ({
    ...buildHotspotFindings(1)[0],
    findingKey: path,
    subject: path,
    evidence: [{ path }],
    metrics,
    ...extra
  }) as Finding

describe('buildHotspotModel', () => {
  it('derives columns from the metric keys that arrived, most frequent first', () => {
    const model = buildHotspotModel(buildHotspotFindings(6))
    expect(model.columns.map((c) => c.key)).toEqual(['authors', 'churn', 'recentFixes'])
    expect(model.columns.map((c) => c.key)).not.toContain('complexity')
  })

  it('uses null for a metric a file does not have', () => {
    const model = buildHotspotModel([
      hot('a.ts', { churn: 3, authors: 1 }),
      hot('b.ts', { churn: 9 })
    ])
    const b = model.rows.find((r) => r.id === 'b.ts')!
    expect(b.values).toEqual([9, null])
  })

  it('caps columns at 6 and keeps the rest in allKeys', () => {
    const metrics = Object.fromEntries(Array.from({ length: 8 }, (_, i) => [`m${i}`, i]))
    const model = buildHotspotModel([hot('a.ts', metrics)])
    expect(model.columns).toHaveLength(6)
    expect(model.allKeys).toHaveLength(8)
    expect(model.allMetrics['a.ts'].m7).toBe(7)
  })

  it('ignores other rules, non-finite numbers and duplicate files', () => {
    const model = buildHotspotModel([
      hot('a.ts', { churn: Number.NaN, authors: 2 }),
      hot('a.ts', { churn: 50 }),
      hot('x.ts', { churn: 1 }, { rule: 'dead_code' })
    ])
    expect(model.rows.map((r) => r.id)).toEqual(['a.ts'])
    expect(model.allKeys).toEqual(['authors'])
  })

  it('returns no rows for no findings and tolerates missing metrics', () => {
    expect(buildHotspotModel([]).rows).toEqual([])
    const bare = { ...hot('a.ts', {}), metrics: undefined } as unknown as Finding
    expect(buildHotspotModel([bare]).rows[0].values).toEqual([])
  })

  it('keeps owner names as plain text and applies the label function', () => {
    const model = buildHotspotModel(buildHotspotFindings(2), (k) => `L:${k}`)
    expect(model.owners['src/area/file-0.ts']).toBe('@platform')
    expect(model.columns[0].label).toBe('L:authors')
  })
})
