import { describe, expect, it } from 'vitest'
import { buildTrendPoint } from '../../../test-support/code-intel-quality-visualization-fake-data'
import { buildQualityTrendModel, TREND_MAX_POINTS } from './quality-trend-series'

describe('buildQualityTrendModel', () => {
  it('keeps a missing diffCoverage as null and converts ratios to percent', () => {
    const points = [
      buildTrendPoint(0, { metrics: { diffCoverage: 0.5 } }),
      buildTrendPoint(1, { metrics: {} }),
      buildTrendPoint(2, { metrics: { diffCoverage: 0 } })
    ]
    expect(buildQualityTrendModel(points).diffCoverageSpark).toEqual([50, null, 0])
  })

  it('returns empty series for no points and a single point for one', () => {
    expect(buildQualityTrendModel([]).xLabels).toEqual([])
    const one = buildQualityTrendModel([buildTrendPoint(0)])
    expect(one.points).toHaveLength(1)
    expect(one.verdictChanges).toBe(0)
  })

  it('orders oldest first even if the backend sends newest first', () => {
    const model = buildQualityTrendModel([
      buildTrendPoint(2),
      buildTrendPoint(0),
      buildTrendPoint(1)
    ])
    expect(model.points.map((p) => p.runIds[0])).toEqual(['run-0', 'run-1', 'run-2'])
  })

  it('marks only the points where the verdict changed', () => {
    const verdicts = ['pass', 'pass', 'warn', 'warn', 'fail'] as const
    const points = verdicts.map((verdict, i) => buildTrendPoint(i, { verdict }))
    const model = buildQualityTrendModel(points)
    const markers = model.series[0].points.map((p) => p.marker ?? null)
    expect(markers).toEqual([null, null, 'warn', null, 'fail'])
    expect(model.verdictChanges).toBe(2)
  })

  it('cuts to the newest 50 points and reports how many were dropped', () => {
    const points = Array.from({ length: 60 }, (_, i) => buildTrendPoint(i))
    const model = buildQualityTrendModel(points)
    expect(model.points).toHaveLength(TREND_MAX_POINTS)
    expect(model.droppedOlder).toBe(10)
    expect(model.points[0].runIds[0]).toBe('run-10')
  })

  it('labels with the turn marker label when turnKey matches, else short commit and UTC time', () => {
    const points = [buildTrendPoint(0), buildTrendPoint(1)]
    const labels = new Map([[points[0].turnKey, 'Turn 1']])
    const model = buildQualityTrendModel(points, { turnLabels: labels })
    expect(model.xLabels[0]).toBe('Turn 1')
    expect(model.xLabels[1]).toBe('a000001 10-01 09:00')
  })

  it('maps unknown source and verdict values safely', () => {
    const model = buildQualityTrendModel([
      buildTrendPoint(0, { source: 'future' as never, verdict: 'odd' as never }),
      buildTrendPoint(1, { source: 'ci' })
    ])
    expect(model.sources).toEqual(['unknown', 'ci'])
    expect(model.verdicts[0]).toBe('unknown')
  })
})
