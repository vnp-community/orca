// @vitest-environment happy-dom
import { act } from 'react'
import { describe, expect, it, vi } from 'vitest'
import { mountChart, registerChartCleanup } from '../../quality-charts/__tests__/chart-test-mount'
import type { UseQualityTrendResult } from '@/hooks/useQualityTrend'
import { buildTrendPoint } from '../../../test-support/code-intel-quality-visualization-fake-data'

const state: { trend: UseQualityTrendResult } = { trend: {} as UseQualityTrendResult }
const useQualityTrend = vi.fn((_wt: string, _group: string) => state.trend)
vi.mock('@/hooks/useQualityTrend', () => ({
  useQualityTrend: (wt: string, group: string) => useQualityTrend(wt, group)
}))

import { QualityTrendPanel } from './QualityTrendPanel'
import type { QualityTrendPanelProps } from './QualityTrendPanel'

registerChartCleanup()
const refetch = vi.fn()
const pointsOf = (n: number) => Array.from({ length: n }, (_, i) => buildTrendPoint(i))

function show(
  patch: Partial<UseQualityTrendResult>,
  props: Partial<QualityTrendPanelProps> = {}
): HTMLElement {
  state.trend = {
    support: 'enabled',
    status: 'ready',
    points: pointsOf(5),
    truncated: false,
    totalCount: 5,
    error: null,
    refetch,
    ...patch
  }
  return mountChart(<QualityTrendPanel worktreeId="wt" onOpenDiff={vi.fn()} {...props} />)
}

describe('QualityTrendPanel', () => {
  it('asks for at least two turns when there is one point and still states its numbers', () => {
    const c = show({ points: pointsOf(1) })
    expect(c.querySelector('[data-trend-need-two]')!.textContent).toContain(
      'At least two turns are needed'
    )
    expect(c.textContent).toContain('Latest point: 12 errors, 8 warnings, 3 info.')
    expect(c.querySelector('svg polyline')).toBeNull()
  })

  it('draws the lines from two points on and marks a verdict change with a diamond label', () => {
    const c = show({ points: [buildTrendPoint(0), buildTrendPoint(1, { verdict: 'fail' })] })
    expect(c.querySelectorAll('[data-series]')).toHaveLength(3)
    expect(c.querySelector('[data-marker="fail"]')).not.toBeNull()
    expect(c.textContent).toContain('◆ Verdict changed')
  })

  it('shows X/Y when the backend has more points than are shown', () => {
    expect(show({ points: pointsOf(50), truncated: true, totalCount: 200 }).textContent).toContain(
      'Showing 50/200'
    )
  })

  it('cuts 51 points to the newest 50 and says so', () => {
    const c = show({ points: pointsOf(51), totalCount: 51 })
    expect(c.textContent).toContain('Showing 50/51')
    expect(c.querySelectorAll('[data-trend-row]')).toHaveLength(50)
  })

  it('does not turn a missing diff coverage into zero', () => {
    const c = show({
      points: [
        buildTrendPoint(0, { metrics: { diffCoverage: 0.5 } }),
        buildTrendPoint(1, { metrics: {} }),
        buildTrendPoint(2, { metrics: { diffCoverage: 0.6 } })
      ]
    })
    expect(c.textContent).toContain('1 points have no diff coverage number.')
    const rows = [...c.querySelectorAll('[data-trend-row]')].map(
      (r) => r.lastElementChild!.textContent
    )
    expect(rows).toEqual(['50%', '—', '60%'])
  })

  it('names the source in words and glyphs, never by colour alone', () => {
    const c = show({
      points: [buildTrendPoint(0, { source: 'local' }), buildTrendPoint(4, { source: 'ci' })]
    })
    const text = c.querySelector('[data-trend-details]')!.textContent
    expect(text).toContain('● Local')
    expect(text).toContain('◇ CI')
    expect(c.textContent).toContain('Figures at HEAD a000004 · CI')
  })

  it('switches the grouping and requests it from the hook', () => {
    const c = show({})
    expect(useQualityTrend).toHaveBeenLastCalledWith('wt', 'turn')
    const commit = [...c.querySelectorAll('button')].find((b) => b.textContent === 'Commit')!
    act(() => commit.click())
    expect(useQualityTrend).toHaveBeenLastCalledWith('wt', 'commit')
    expect(c.textContent).toContain('Trend by commit')
  })

  it('has no selectable points when the turn comparison action does not exist', () => {
    const c = show({})
    expect(c.querySelectorAll('rect[data-grid-cell]')).toHaveLength(0)
    expect(c.textContent).not.toContain('Select a turn')
  })

  it('compares a known turn on selection and ignores unknown turns', () => {
    const points = pointsOf(3)
    const onCompareTurn = vi.fn()
    const c = show(
      { points },
      { turnLabels: new Map([[points[1].turnKey, 'Turn 2']]), onCompareTurn }
    )
    const cells = c.querySelectorAll('rect[data-grid-cell]')
    expect(cells).toHaveLength(3)
    const grid = c.querySelector('svg[role="grid"]')!
    const press = (key: string) =>
      act(
        () =>
          void grid.dispatchEvent(
            new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true })
          )
      )
    press('Enter')
    expect(onCompareTurn).not.toHaveBeenCalled()
    press('ArrowRight')
    press('Enter')
    expect(onCompareTurn).toHaveBeenCalledWith(points[1].turnKey)
  })

  it('renders empty, loading, error with retry and stale', () => {
    expect(show({ status: 'empty', points: [] }).textContent).toContain(
      'No quality runs have been recorded'
    )
    expect(
      show({ status: 'loading', points: [] }).querySelector('[data-status="loading"]')
    ).not.toBeNull()
    const error = show({ status: 'error', points: [], error: { kind: 'too-large' } as never })
    expect(error.textContent).toContain('too large')
    act(() => (error.querySelector('[data-chart-error] button') as HTMLButtonElement).click())
    expect(refetch).toHaveBeenCalled()
    expect(
      show({ status: 'stale' }).querySelector('[data-quality-trend] [data-chart-stale]')
    ).not.toBeNull()
  })

  it('does not claim improvement', () => {
    const text = show({}).textContent!.toLowerCase()
    expect(text).not.toContain('improv')
    expect(text).not.toContain('worse')
  })

  it('renders nothing when quality is unsupported', () => {
    expect(show({ support: 'unsupported' }).innerHTML).toBe('')
  })
})
