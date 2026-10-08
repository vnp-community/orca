// @vitest-environment happy-dom

import { describe, expect, it } from 'vitest'
import { StackedSeverityBar } from '../StackedSeverityBar'
import { mountChart, registerChartCleanup } from './chart-test-mount'

registerChartCleanup()
const frame = { id: 's', title: 'Findings by severity' }

describe('StackedSeverityBar', () => {
  it('shows number, label and glyph per non-empty segment', () => {
    const c = mountChart(
      <StackedSeverityBar frame={frame} counts={{ error: 3, warning: 0, info: 5 }} ranCheck />
    )
    const segments = c.querySelectorAll('[data-stacked-bar] [data-level]')
    expect([...segments].map((s) => s.getAttribute('data-level'))).toEqual(['error', 'info'])
    expect(segments[0].textContent).toContain('Error')
    expect(segments[0].textContent).toContain('3')
    expect(segments[0].querySelector('svg[data-shape="octagon"]')).not.toBeNull()
    // zero segment is still in the alternative table
    expect(c.querySelector('[data-chart-table]')?.textContent).toContain('Warning')
  })

  it('does not call zero findings clean unless checks ran', () => {
    const ran = mountChart(
      <StackedSeverityBar frame={frame} counts={{ error: 0, warning: 0, info: 0 }} ranCheck />
    )
    expect(ran.textContent).toContain('No findings in the checks that ran')
    const notRan = mountChart(
      <StackedSeverityBar frame={frame} counts={{ error: 0, warning: 0, info: 0 }} />
    )
    expect(notRan.textContent).toContain('No data yet')
    expect(notRan.textContent).not.toContain('No findings')
  })

  it('sanitises invalid counts', () => {
    const c = mountChart(
      <StackedSeverityBar frame={frame} counts={{ error: Number.NaN, warning: -2, info: 1 }} />
    )
    expect(c.innerHTML).not.toContain('NaN')
  })
})
