// @vitest-environment happy-dom

import { describe, expect, it } from 'vitest'
import { DiffCoverageGauge } from '../DiffCoverageGauge'
import { mountChart, registerChartCleanup } from './chart-test-mount'

registerChartCleanup()
const frame = { id: 'g', title: 'Diff coverage' }

describe('DiffCoverageGauge', () => {
  it('shows 62% with two distinct threshold markers', () => {
    const c = mountChart(
      <DiffCoverageGauge
        frame={frame}
        covered={62}
        total={100}
        thresholds={{ warnBelow: 80, failBelow: 50 }}
        source="measured"
      />
    )
    const meter = c.querySelector('[role="meter"]')!
    expect(meter.getAttribute('aria-valuenow')).toBe('62')
    expect(meter.getAttribute('aria-valuetext')).toContain('62% (62/100 changed lines covered)')
    const warn = c.querySelector('[data-threshold="warn"]')!
    const fail = c.querySelector('[data-threshold="fail"]')!
    expect(warn.className).toContain('border-dashed')
    expect(fail.className).toContain('border-solid')
    expect(c.textContent).toContain('Below the warn threshold')
    expect(c.textContent).toContain('Measured')
  })

  it('works without thresholds (null)', () => {
    const c = mountChart(
      <DiffCoverageGauge
        frame={frame}
        covered={5}
        total={10}
        thresholds={{ warnBelow: null, failBelow: null }}
        source="measured"
      />
    )
    expect(c.querySelector('[data-threshold]')).toBeNull()
  })

  it('marks estimated coverage and partial scope in words', () => {
    const c = mountChart(
      <DiffCoverageGauge frame={frame} covered={5} total={10} source="estimated" partial />
    )
    expect(c.textContent).toContain('Estimated from test edges, not measured coverage')
    expect(c.textContent).toContain('Partial scope')
  })

  it('total 0 shows no data, never 0%', () => {
    const c = mountChart(
      <DiffCoverageGauge frame={frame} covered={0} total={0} source="measured" />
    )
    expect(c.textContent).toContain('No coverage data for this scope yet')
    expect(c.textContent).not.toContain('0%')
    expect(c.querySelector('[role="meter"]')).toBeNull()
    expect(c.innerHTML).not.toContain('NaN')
  })

  it('caps covered > total at 100% and records the anomaly', () => {
    const c = mountChart(
      <DiffCoverageGauge frame={frame} covered={12} total={10} source="measured" />
    )
    expect(c.querySelector('[role="meter"]')?.getAttribute('aria-valuenow')).toBe('100')
    expect(c.textContent).toContain('Covered exceeds total; value capped at 100%')
    expect(c.querySelector('[data-chart-table]')?.textContent).toContain('capped')
  })
})
