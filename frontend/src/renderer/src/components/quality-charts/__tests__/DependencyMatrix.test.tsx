// @vitest-environment happy-dom

import { act } from 'react'
import { describe, expect, it, vi } from 'vitest'
import { DependencyMatrix } from '../DependencyMatrix'
import { mountChart, registerChartCleanup } from './chart-test-mount'

registerChartCleanup()
const frame = { id: 'dm', title: 'Module dependencies' }
const nodes = ['a', 'b', 'c', 'd'].map((id) => ({ id, label: `mod-${id}` }))

describe('DependencyMatrix', () => {
  it('orders a DAG with dependencies above the diagonal and no cycle markers', () => {
    const edges = [
      { from: 'a', to: 'b', weight: 2 },
      { from: 'b', to: 'c', weight: 1 }
    ]
    const c = mountChart(<DependencyMatrix frame={frame} nodes={nodes} edges={edges} />)
    expect(c.querySelectorAll('[role="gridcell"]')).toHaveLength(2)
    expect(c.querySelector('[data-cycle-block]')).toBeNull()
    expect(c.querySelector('[data-backward]')).toBeNull()
  })

  it('marks cycles with a framed block and a triangle, not only colour', () => {
    const edges = [
      { from: 'a', to: 'b', weight: 1 },
      { from: 'b', to: 'a', weight: 1 }
    ]
    const c = mountChart(<DependencyMatrix frame={frame} nodes={nodes} edges={edges} />)
    expect(c.querySelector('[data-cycle-block="a b"]')).not.toBeNull()
    expect(c.querySelectorAll('[data-backward]')).toHaveLength(1)
    expect(c.querySelector('[data-grid-cell][aria-label*="Backward dependency"]')).not.toBeNull()
    expect(c.textContent).toContain('▲ Backward dependency')
  })

  it('exposes 1-based indexes and one Tab stop, and selects a cell', () => {
    const onSelectCell = vi.fn()
    const edges = [
      { from: 'a', to: 'b', weight: 2 },
      { from: 'b', to: 'c', weight: 1 }
    ]
    const c = mountChart(
      <DependencyMatrix frame={frame} nodes={nodes} edges={edges} onSelectCell={onSelectCell} />
    )
    const first = c.querySelector('[role="gridcell"][tabindex="0"]')!
    expect(first.getAttribute('aria-rowindex')).toBe('1')
    expect(first.getAttribute('aria-colindex')).toBe('2')
    expect(c.querySelectorAll('[role="gridcell"][tabindex="0"]')).toHaveLength(1)
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
    expect(onSelectCell).toHaveBeenCalledWith('b', 'c')
  })

  it('collapses extra nodes into one trailing Other row with X/Y', () => {
    const many = Array.from({ length: 10 }, (_, i) => ({ id: `n${i}`, label: `n${i}` }))
    const edges = many.slice(1).map((n, i) => ({ from: many[i].id, to: n.id, weight: i + 1 }))
    const c = mountChart(<DependencyMatrix frame={frame} nodes={many} edges={edges} maxNodes={5} />)
    expect(c.textContent).toContain('Other (6)')
    expect(c.textContent).toContain('Showing 4/10')
    expect(c.querySelectorAll('svg[data-matrix] text').length).toBeGreaterThan(0)
  })

  it('shows a reasoned empty state for empty graphs', () => {
    const c = mountChart(<DependencyMatrix frame={frame} nodes={[]} edges={[]} />)
    expect(c.querySelector('[data-chart-empty]')?.textContent).toContain('no dependencies')
    expect(c.innerHTML).not.toContain('NaN')
  })

  it('handles self loops without NaN', () => {
    const c = mountChart(
      <DependencyMatrix frame={frame} nodes={nodes} edges={[{ from: 'a', to: 'a', weight: 1 }]} />
    )
    expect(c.innerHTML).not.toContain('NaN')
    expect(c.querySelector('[data-cycle-block="a"]')).not.toBeNull()
  })
})
