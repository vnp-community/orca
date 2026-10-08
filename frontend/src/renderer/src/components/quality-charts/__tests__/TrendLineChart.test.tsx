// @vitest-environment happy-dom

import { act } from 'react'
import { describe, expect, it, vi } from 'vitest'
import { TrendLineChart, type TrendSeries } from '../TrendLineChart'
import { mountChart, registerChartCleanup } from './chart-test-mount'

registerChartCleanup()
const frame = { id: 't', title: 'Findings per turn' }

function series(values: (number | null)[]): TrendSeries[] {
  return [
    { id: 'error', label: 'Errors', points: values.map((value, i) => ({ label: `T${i}`, value })) },
    {
      id: 'warning',
      label: 'Warnings',
      points: values.map((value, i) => ({
        label: `T${i}`,
        value: value === null ? null : value + 1
      }))
    }
  ]
}

describe('TrendLineChart', () => {
  it('breaks the line at null and never draws NaN', () => {
    const labels = ['T0', 'T1', 'T2', 'T3', 'T4']
    const c = mountChart(
      <TrendLineChart frame={frame} series={series([1, 2, null, 4, 5])} xLabels={labels} />
    )
    expect(c.querySelectorAll('[data-series="error"] polyline')).toHaveLength(2)
    expect(c.innerHTML).not.toContain('NaN')
    const cells = [...c.querySelectorAll('[data-chart-table] tbody tr')][2].querySelectorAll('td')
    expect(cells[1].textContent).toBe('—')
  })

  it('renders all-null series without NaN', () => {
    const c = mountChart(
      <TrendLineChart frame={frame} series={series([null, null])} xLabels={['a', 'b']} />
    )
    expect(c.innerHTML).not.toContain('NaN')
    expect(c.querySelectorAll('polyline')).toHaveLength(0)
  })

  it('keeps the newest maxPoints and says how many are shown', () => {
    const values = Array.from({ length: 80 }, (_, i) => i)
    const labels = values.map((i) => `T${i}`)
    const c = mountChart(<TrendLineChart frame={frame} series={series(values)} xLabels={labels} />)
    expect(c.textContent).toContain('Showing 50/80')
    expect(c.querySelectorAll('[data-chart-table] tbody tr')).toHaveLength(50)
    expect(c.querySelector('[data-chart-table] tbody tr td')?.textContent).toBe('T30')
  })

  it('labels verdict changes with text, not only a shape', () => {
    const s = series([1, 2, 3])
    s[0].points[1].marker = 'fail'
    const c = mountChart(<TrendLineChart frame={frame} series={s} xLabels={['a', 'b', 'c']} />)
    expect(c.querySelector('[data-marker="fail"]')?.textContent).toBe('Fail')
    expect(c.textContent).toContain('Verdict changed')
  })

  it('uses a distinct dash per severity series', () => {
    const c = mountChart(
      <TrendLineChart frame={frame} series={series([1, 2, 3])} xLabels={['a', 'b', 'c']} />
    )
    const dashes = [...c.querySelectorAll('polyline')].map((p) =>
      p.getAttribute('stroke-dasharray')
    )
    expect(new Set(dashes).size).toBe(2)
  })

  it('makes points keyboard selectable as a single tab stop', () => {
    const onSelect = vi.fn()
    const c = mountChart(
      <TrendLineChart
        frame={frame}
        series={series([1, 2, 3])}
        xLabels={['a', 'b', 'c']}
        onSelectPoint={onSelect}
      />
    )
    const cells = [...c.querySelectorAll('[role="gridcell"]')]
    expect(cells).toHaveLength(3)
    expect(cells.filter((x) => x.getAttribute('tabindex') === '0')).toHaveLength(1)
    const grid = c.querySelector('[role="grid"]')!
    act(() => {
      grid.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true }))
    })
    act(() => {
      grid.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }))
    })
    expect(onSelect).toHaveBeenCalledWith(1)
  })
})
