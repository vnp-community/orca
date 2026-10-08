// @vitest-environment happy-dom

import { describe, expect, it } from 'vitest'
import { SparklineChart } from '../SparklineChart'
import { mountChart, registerChartCleanup } from './chart-test-mount'

registerChartCleanup()

describe('SparklineChart', () => {
  it('splits the line at null values', () => {
    const c = mountChart(<SparklineChart label="Errors" points={[1, 2, null, 4, 5]} />)
    expect(c.querySelectorAll('polyline')).toHaveLength(2)
    expect(c.querySelector('[role="img"]')?.getAttribute('aria-label')).toContain('Errors')
    expect(c.innerHTML).not.toContain('NaN')
  })

  it('needs two numeric points', () => {
    const one = mountChart(<SparklineChart label="E" points={[null, 3]} />)
    expect(one.querySelector('polyline')).toBeNull()
    expect(one.textContent).toContain('At least two points are needed')
    const none = mountChart(<SparklineChart label="E" points={[null, null]} />)
    expect(none.innerHTML).not.toContain('NaN')
  })

  it('wraps in a ChartFrame when asked', () => {
    const c = mountChart(
      <SparklineChart label="E" points={[1, 2, 3]} frame={{ id: 'sp', title: 'E trend' }} />
    )
    expect(c.querySelector('figure')).not.toBeNull()
    expect(c.querySelector('table')).not.toBeNull()
  })
})
