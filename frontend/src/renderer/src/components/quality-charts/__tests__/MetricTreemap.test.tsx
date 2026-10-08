// @vitest-environment happy-dom

import { act } from 'react'
import { describe, expect, it, vi } from 'vitest'
import { MetricTreemap, type TreemapItem } from '../MetricTreemap'
import { mountChart, registerChartCleanup } from './chart-test-mount'

registerChartCleanup()
const frame = { id: 'tm', title: 'Hotspots by size' }

function items(n: number): TreemapItem[] {
  return Array.from({ length: n }, (_, i) => ({
    id: `f${i}`,
    label: `file-${i}.ts`,
    size: 1 + ((i * 37) % 100),
    intensity: (i * 13) % 50
  }))
}

describe('MetricTreemap', () => {
  it('renders one button per tile with a full aria-label', () => {
    const c = mountChart(
      <MetricTreemap frame={frame} items={items(5)} sizeLabel="lines" intensityLabel="findings" />
    )
    const tiles = c.querySelectorAll('button[role="gridcell"]')
    expect(tiles).toHaveLength(5)
    expect(tiles[0].getAttribute('aria-label')).toMatch(/^file-\d+\.ts, lines: \d+, findings: \d+$/)
    expect(c.querySelectorAll('[tabindex="0"][role="gridcell"]')).toHaveLength(1)
  })

  it('caps at maxTiles, adds one +N tile and says X/Y', () => {
    const c = mountChart(
      <MetricTreemap
        frame={frame}
        items={items(30)}
        sizeLabel="lines"
        intensityLabel="findings"
        maxTiles={10}
      />
    )
    expect(c.querySelectorAll('button[role="gridcell"]')).toHaveLength(10)
    expect(c.querySelector('[data-more]')?.textContent).toBe('+20 more')
    expect(c.textContent).toContain('Showing 10/30')
  })

  it('selects a tile with the keyboard', () => {
    const onSelect = vi.fn()
    const c = mountChart(
      <MetricTreemap
        frame={frame}
        items={items(4)}
        sizeLabel="lines"
        intensityLabel="findings"
        onSelect={onSelect}
      />
    )
    const grid = c.querySelector('[role="grid"]')!
    act(() => {
      grid.dispatchEvent(
        new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true, cancelable: true })
      )
    })
    act(() => {
      grid.dispatchEvent(
        new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true })
      )
    })
    expect(onSelect).toHaveBeenCalledTimes(1)
  })

  it('does not throw on zero, negative or NaN sizes and empty input', () => {
    const bad: TreemapItem[] = [
      { id: 'a', label: 'a', size: 0, intensity: 1 },
      { id: 'b', label: 'b', size: -3, intensity: Number.NaN },
      { id: 'c', label: 'c', size: Number.NaN, intensity: 1 }
    ]
    const c = mountChart(
      <MetricTreemap frame={frame} items={bad} sizeLabel="lines" intensityLabel="findings" />
    )
    expect(c.querySelector('[data-chart-empty]')?.textContent).toContain('no items')
    expect(c.innerHTML).not.toContain('NaN')
  })

  it('draws overlay as a border style plus text, and heat as token variables', () => {
    const list = items(3)
    list[0].overlay = ['changed']
    const c = mountChart(
      <MetricTreemap frame={frame} items={list} sizeLabel="lines" intensityLabel="findings" />
    )
    const tiles = [...c.querySelectorAll('button[role="gridcell"]')]
    const marked = tiles.find((t) => t.getAttribute('data-overlay'))!
    expect(marked.className).toContain('border-dashed')
    expect(marked.getAttribute('aria-label')).toContain('changed')
    expect(c.innerHTML).toMatch(/var\(--quality-heat-\d\)/)
  })
})
