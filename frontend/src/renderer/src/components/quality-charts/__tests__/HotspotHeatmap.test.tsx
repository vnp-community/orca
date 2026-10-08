// @vitest-environment happy-dom

import { act } from 'react'
import { describe, expect, it, vi } from 'vitest'
import { HotspotHeatmap } from '../HotspotHeatmap'
import { mountChart, registerChartCleanup } from './chart-test-mount'

registerChartCleanup()
const frame = { id: 'hm', title: 'Hotspots' }
const columns = [
  { key: 'churn', label: 'Churn' },
  { key: 'unc', label: 'Uncovered', unit: '%' }
]
const rows = [
  { id: 'a', label: 'src/a.ts', values: [18, 12] },
  { id: 'b', label: 'src/b.ts', values: [3, null] },
  { id: 'c', label: 'src/c.ts', values: [9, 41] }
]

describe('HotspotHeatmap', () => {
  it('always shows the number in the cell and a dash for null', () => {
    const c = mountChart(<HotspotHeatmap frame={frame} rows={rows} columns={columns} />)
    const cells = [...c.querySelectorAll('[role="gridcell"]')].map((x) => x.textContent)
    expect(cells).toEqual(['18', '12%', '3', '—', '9', '41%'])
    const nullCell = c.querySelector('[data-grid-cell="1:1"]')!
    expect(nullCell.getAttribute('data-intensity')).toBe('')
  })

  it('has one Tab stop and 1-based row/col indexes', () => {
    const c = mountChart(<HotspotHeatmap frame={frame} rows={rows} columns={columns} />)
    expect(c.querySelectorAll('[role="gridcell"][tabindex="0"]')).toHaveLength(1)
    const cell = c.querySelector('[data-grid-cell="2:1"]')!
    expect(cell.getAttribute('aria-rowindex')).toBe('3')
    expect(cell.getAttribute('aria-colindex')).toBe('2')
  })

  it('selects a row with Enter', () => {
    const onSelectRow = vi.fn()
    const c = mountChart(
      <HotspotHeatmap frame={frame} rows={rows} columns={columns} onSelectRow={onSelectRow} />
    )
    const grid = c.querySelector('[role="grid"]')!
    act(() => {
      grid.dispatchEvent(
        new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true, cancelable: true })
      )
    })
    act(() => {
      grid.dispatchEvent(
        new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true })
      )
    })
    expect(onSelectRow).toHaveBeenCalledWith('b')
  })

  it('cuts at maxRows and says X/Y', () => {
    const c = mountChart(<HotspotHeatmap frame={frame} rows={rows} columns={columns} maxRows={2} />)
    expect(c.querySelectorAll('[role="row"]')).toHaveLength(3) // header + 2
    expect(c.textContent).toContain('Showing 2/3')
  })

  it('shows an empty reason for no rows', () => {
    const c = mountChart(<HotspotHeatmap frame={frame} rows={[]} columns={columns} />)
    expect(c.querySelector('[data-chart-empty]')).not.toBeNull()
  })
})
